// Copyright (c) 2019 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"
	"github.com/sylabs/wlm-operator/pkg/slurm"
	"github.com/sylabs/wlm-operator/pkg/workload/api"
)

const (
	pullAndRunBatchScriptT = `#!/bin/sh
	srun singularity pull -U --name %[2]s %[1]s
	srun singularity run %[2]s
	srun rm %[2]s`
	runBatchScriptT = `#!/bin/sh
	srun singularity run %s`
)

type (
	// Slurm implements WorkloadManagerServer.
	Slurm struct {
		uid    int64
		cfg    Config
		client *slurm.Client
	}

	// Config is a red-box configuration for each partition available.
	Config map[string]PartitionResources

	// PartitionResources configure how red-box will see slurm partition resources.
	// In auto mode red-box will attempt to query partition resources from slurm, but
	// administrator can set up them manually.
	PartitionResources struct {
		AutoNodes      bool `yaml:"auto_nodes"`
		AutoCPUPerNode bool `yaml:"auto_cpu_per_node"`
		AutoMemPerNode bool `yaml:"auto_mem_per_node"`
		AutoWallTime   bool `yaml:"auto_wall_time"`

		Nodes      int64         `yaml:"nodes"`
		CPUPerNode int64         `yaml:"cpu_per_node"`
		MemPerNode int64         `yaml:"mem_per_node"`
		WallTime   time.Duration `yaml:"wall_time"`

		AdditionalFeatures []Feature `yaml:"additional_features"`
	}

	// Feature represents slurm partition feature.
	Feature struct {
		Name     string `yaml:"name"`
		Version  string `yaml:"version"`
		Quantity int64  `yaml:"quantity"`
	}
)

// NewSlurm creates a new instance of Slurm.
func NewSlurm(c *slurm.Client, cfg Config) *Slurm {
	return &Slurm{client: c, cfg: cfg, uid: int64(os.Geteuid())}
}

// SubmitJob submits job and returns id of it in case of success.
func (s *Slurm) SubmitJob(ctx context.Context, req *api.SubmitJobRequest) (*api.SubmitJobResponse, error) {
	// todo use client id from req
	id, err := s.client.SBatch(req.Script, req.Partition)
	if err != nil {
		return nil, errors.Wrap(err, "could not submit sbatch script")
	}

	return &api.SubmitJobResponse{
		JobId: id,
	}, nil
}

// SubmitJobContainer starts a container from the provided image name inside a sbatch script.
func (s *Slurm) SubmitJobContainer(ctx context.Context, r *api.SubmitJobContainerRequest) (*api.SubmitJobContainerResponse, error) {
	script := ""
	// checks if sif is located somewhere on the host machine
	if strings.HasPrefix(r.ImageName, "file://") {
		image := strings.TrimPrefix(r.ImageName, "file://")
		script = fmt.Sprintf(runBatchScriptT, image)
	} else {
		script = fmt.Sprintf(pullAndRunBatchScriptT, r.ImageName, uuid.New())
	}

	id, err := s.client.SBatch(script, r.Partition)
	if err != nil {
		return nil, errors.Wrap(err, "could not submit sbatch script")
	}

	return &api.SubmitJobContainerResponse{
		JobId: id,
	}, nil
}

// CancelJob cancels job.
func (s *Slurm) CancelJob(ctx context.Context, req *api.CancelJobRequest) (*api.CancelJobResponse, error) {
	if err := s.client.SCancel(req.JobId); err != nil {
		return nil, errors.Wrapf(err, "could not cancel job %d", req.JobId)
	}

	return &api.CancelJobResponse{}, nil
}

// JobInfo returns information about a job from 'scontrol show jobid'.
// Safe to call before job finished. After it could return an error.
func (s *Slurm) JobInfo(ctx context.Context, req *api.JobInfoRequest) (*api.JobInfoResponse, error) {
	info, err := s.client.SJobInfo(req.JobId)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get job %d info", req.JobId)
	}

	pInfo, err := mapSInfoToProtoInfo(info)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert slurm info into proto info")
	}

	if len(pInfo) == 0 {
		return nil, errors.New("job info slice is empty, probably invalid scontrol output")
	}

	return &api.JobInfoResponse{Info: pInfo}, nil
}

// JobSteps returns information about job steps from 'sacct'.
// Safe to call after job started. Before it could return an error.
func (s *Slurm) JobSteps(ctx context.Context, req *api.JobStepsRequest) (*api.JobStepsResponse, error) {
	steps, err := s.client.SJobSteps(req.JobId)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get job %d steps", req.JobId)
	}

	pSteps, err := toProtoSteps(steps)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert slurm steps into proto steps")
	}

	return &api.JobStepsResponse{JobSteps: pSteps}, nil
}

// OpenFile opens requested file and return chunks with bytes.
func (s *Slurm) OpenFile(r *api.OpenFileRequest, req api.WorkloadManager_OpenFileServer) error {
	fd, err := s.client.Open(r.Path)
	if err != nil {
		return errors.Wrapf(err, "could not open file at %s", r.Path)
	}
	defer fd.Close()

	buff := make([]byte, 128)
	for {
		n, err := fd.Read(buff)
		if n > 0 {
			if err := req.Send(&api.Chunk{Content: buff[:n]}); err != nil {
				return errors.Wrap(err, "could not send chunk")
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}
	}

	return nil
}

// TailFile tails a file till close requested.
// To start receiving file bytes client should send a request with file path and action start,
// to stop client should send a request with action readToEndAndClose (file path is not required)
// and after reaching end method will send EOF error.
func (s *Slurm) TailFile(req api.WorkloadManager_TailFileServer) error {
	r, err := req.Recv()
	if err != nil {
		return errors.Wrap(err, "could not receive request")
	}

	fd, err := s.client.Tail(r.Path)
	if err != nil {
		return errors.Wrapf(err, "could not tail file at %s", r.Path)
	}
	defer func(p string) {
		log.Printf("Tail file at %s finished", p)
	}(r.Path)

	requestCh := make(chan *api.TailFileRequest)
	go func() {
		r, err := req.Recv()
		if err != nil {
			if err != io.EOF {
				log.Printf("could not recive request err: %s", err)
			}
			return
		}

		requestCh <- r
	}()

	buff := make([]byte, 128)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-req.Context().Done():
			return req.Context().Err()
		case r := <-requestCh:
			if r.Action == api.TailAction_ReadToEndAndClose {
				_ = fd.Close()
			}
		case <-ticker.C:
			n, err := fd.Read(buff)
			if err != nil && n == 0 {
				return err
			}

			if n == 0 {
				continue
			}

			if err := req.Send(&api.Chunk{Content: buff[:n]}); err != nil {
				return errors.Wrap(err, "could not send chunk")
			}
		}
	}
}

// Resources return available resources on slurm cluster in a requested partition.
func (s *Slurm) Resources(_ context.Context, req *api.ResourcesRequest) (*api.ResourcesResponse, error) {
	slurmResources, err := s.client.Resources(req.Partition)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get resources for partition %s", req.Partition)
	}

	partitionResources := s.cfg[req.Partition]
	response := &api.ResourcesResponse{
		Nodes:      partitionResources.Nodes,
		CpuPerNode: partitionResources.CPUPerNode,
		MemPerNode: partitionResources.MemPerNode,
		WallTime:   int64(partitionResources.WallTime.Seconds()),
	}

	for _, f := range slurmResources.Features {
		response.Features = append(response.Features, &api.Feature{
			Name:     f.Name,
			Version:  f.Version,
			Quantity: f.Quantity,
		})
	}
	for _, f := range partitionResources.AdditionalFeatures {
		response.Features = append(response.Features, &api.Feature{
			Name:     f.Name,
			Version:  f.Version,
			Quantity: f.Quantity,
		})
	}

	if partitionResources.AutoNodes || response.Nodes == 0 {
		response.Nodes = slurmResources.Nodes
	}
	if partitionResources.AutoCPUPerNode || response.CpuPerNode == 0 {
		response.CpuPerNode = slurmResources.CPUPerNode
	}
	if partitionResources.AutoMemPerNode || response.MemPerNode == 0 {
		response.MemPerNode = slurmResources.MemPerNode
	}
	if partitionResources.AutoWallTime || response.WallTime == 0 {
		response.WallTime = int64(slurmResources.WallTime.Seconds())
	}

	return response, nil
}

// Partitions returns partition names.
func (s *Slurm) Partitions(context.Context, *api.PartitionsRequest) (*api.PartitionsResponse, error) {
	names, err := s.client.Partitions()
	if err != nil {
		return nil, errors.Wrap(err, "could not get partition names")
	}

	return &api.PartitionsResponse{Partition: names}, nil
}

// WorkloadInfo returns wlm info (name, version, red-box uid)
func (s *Slurm) WorkloadInfo(context.Context, *api.WorkloadInfoRequest) (*api.WorkloadInfoResponse, error) {
	const wlmName = "slurm"

	sVersion, err := s.client.Version()
	if err != nil {
		return nil, errors.Wrap(err, "could not get slurm version")
	}

	return &api.WorkloadInfoResponse{
		Name:    wlmName,
		Version: sVersion,
		Uid:     s.uid,
	}, nil
}

func toProtoSteps(ss []*slurm.JobStepInfo) ([]*api.JobStepInfo, error) {
	pSteps := make([]*api.JobStepInfo, len(ss))

	for i, s := range ss {
		var startedAt *timestamp.Timestamp
		if s.StartedAt != nil {
			pt, err := ptypes.TimestampProto(*s.StartedAt)
			if err != nil {
				return nil, errors.Wrap(err, "could not convert started go time to proto time")
			}

			startedAt = pt
		}

		var finishedAt *timestamp.Timestamp
		if s.FinishedAt != nil {
			pt, err := ptypes.TimestampProto(*s.FinishedAt)
			if err != nil {
				return nil, errors.Wrap(err, "could not convert finished go time to proto time")
			}

			finishedAt = pt
		}

		status, ok := api.JobStatus_value[s.State]
		if !ok {
			status = int32(api.JobStatus_UNKNOWN)
		}

		pSteps[i] = &api.JobStepInfo{
			Id:        s.ID,
			Name:      s.Name,
			ExitCode:  int32(s.ExitCode),
			Status:    api.JobStatus(status),
			StartTime: startedAt,
			EndTime:   finishedAt,
		}
	}

	return pSteps, nil
}

func mapSInfoToProtoInfo(si []*slurm.JobInfo) ([]*api.JobInfo, error) {
	pInfs := make([]*api.JobInfo, len(si))
	for i, inf := range si {
		var submitTime *timestamp.Timestamp
		if inf.SubmitTime != nil {
			pt, err := ptypes.TimestampProto(*inf.SubmitTime)
			if err != nil {
				return nil, errors.Wrap(err, "could not convert submit go time to proto time")
			}

			submitTime = pt
		}

		var startTime *timestamp.Timestamp
		if inf.StartTime != nil {
			pt, err := ptypes.TimestampProto(*inf.StartTime)
			if err != nil {
				return nil, errors.Wrap(err, "could not convert start go time to proto time")
			}

			startTime = pt
		}

		var runTime *duration.Duration
		if inf.RunTime != nil {
			runTime = ptypes.DurationProto(*inf.RunTime)
		}

		var timeLimit *duration.Duration
		if inf.TimeLimit != nil {
			timeLimit = ptypes.DurationProto(*inf.TimeLimit)
		}

		status, ok := api.JobStatus_value[inf.State]
		if !ok {
			status = int32(api.JobStatus_UNKNOWN)
		}

		pi := api.JobInfo{
			Id:         inf.ID,
			UserId:     inf.UserID,
			Name:       inf.Name,
			ExitCode:   inf.ExitCode,
			Status:     api.JobStatus(status),
			SubmitTime: submitTime,
			StartTime:  startTime,
			RunTime:    runTime,
			TimeLimit:  timeLimit,
			WorkingDir: inf.WorkDir,
			StdOut:     inf.StdOut,
			StdErr:     inf.StdErr,
			Partition:  inf.Partition,
			NodeList:   inf.NodeList,
			BatchHost:  inf.BatchHost,
			NumNodes:   inf.NumNodes,
			ArrayId:    inf.ArrayJobID,
		}
		pInfs[i] = &pi
	}

	return pInfs, nil
}
