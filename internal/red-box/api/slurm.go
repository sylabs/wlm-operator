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
	"io"
	"log"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/pkg/slurm"
	"github.com/sylabs/slurm-operator/pkg/workload/api"
)

type Feature struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	Quantity int64  `yaml:"quantity"`
}

type Resources struct {
	AutoNodes bool  `yaml:"auto_nodes"`
	Nodes     int64 `yaml:"nodes"`

	AutoCpuPerNode bool  `yaml:"auto_cpu_per_node"`
	CpuPerNode     int64 `yaml:"cpu_per_node"`

	AutoMemPerNode bool  `yaml:"auto_mem_per_node"`
	MemPerNode     int64 `yaml:"mem_per_node"`

	AutoWallTime bool          `yaml:"auto_wall_time"`
	WallTime     time.Duration `yaml:"wall_time"`

	AdditionalFeatures []*Feature `yaml:"additional_features"`
}

type Config struct {
	Partition string    `yaml:"partition"`
	Resources Resources `yaml:"resources"`
}

// Slurm implements WorkloadManagerServer
type Slurm struct {
	cfg    *Config
	client *slurm.Client
}

// NewSlurmAPI creates a new instance of Slurm
func NewSlurmAPI(c *slurm.Client, cfg *Config) *Slurm {
	return &Slurm{client: c, cfg: cfg}
}

// SubmitJob submits job and returns id of it in case of success
func (a *Slurm) SubmitJob(ctx context.Context, r *api.SubmitJobRequest) (*api.SubmitJobResponse, error) {
	id, err := a.client.SBatch(r.Script)
	if err != nil {
		return nil, errors.Wrap(err, "can't submit sbatch script")
	}

	return &api.SubmitJobResponse{
		JobId: id,
	}, nil
}

// CancelJob cancels job
func (a *Slurm) CancelJob(ctx context.Context, r *api.CancelJobRequest) (*api.CancelJobResponse, error) {
	if err := a.client.SCancel(r.JobId); err != nil {
		return nil, errors.Wrapf(err, "can't cancel job %d", r.JobId)
	}

	return &api.CancelJobResponse{}, nil
}

// JobInfo returns information about a job from 'scontrol show jobid'
// Safe to call before job finished. After it could return an error
func (a *Slurm) JobInfo(ctx context.Context, r *api.JobInfoRequest) (*api.JobInfoResponse, error) {
	info, err := a.client.SJobInfo(r.JobId)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get job %d info", r.JobId)
	}

	pInfo, err := mapSInfoToProtoInfo(info)
	if err != nil {
		return nil, errors.Wrap(err, "can't convert slurm info into proto info")
	}

	if len(pInfo) == 0 {
		return nil, errors.New("job info slice is empty, probably invalid scontrol output")
	}

	return &api.JobInfoResponse{Info: pInfo}, nil
}

// JobSteps returns information about job steps from 'sacct'
// Safe to call after job started. Before it could return an error
func (a *Slurm) JobSteps(ctx context.Context, r *api.JobStepsRequest) (*api.JobStepsResponse, error) {
	steps, err := a.client.SJobSteps(r.JobId)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get job %d steps", r.JobId)
	}

	pSteps, err := mapSStepsToProtoSteps(steps)
	if err != nil {
		return nil, errors.Wrap(err, "can't convert slurm steps into proto steps")
	}

	return &api.JobStepsResponse{JobSteps: pSteps}, nil
}

// OpenFile opens requested file and return chunks with bytes
func (a *Slurm) OpenFile(r *api.OpenFileRequest, s api.WorkloadManager_OpenFileServer) error {
	fd, err := a.client.Open(r.Path)
	if err != nil {
		return errors.Wrapf(err, "can't open file at %s", r.Path)
	}
	defer fd.Close()

	buff := make([]byte, 128)
	for {
		n, err := fd.Read(buff)
		if n > 0 {
			if err := s.Send(&api.Chunk{Content: buff[:n]}); err != nil {
				return errors.Wrap(err, "can't send chunk")
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

// TailFile tails a file till close requested
// To start receiving file bytes client should send a request with file path and action start,
// to stop client should send a request with action readToEndAndClose (file path is not required)
//  and after reaching end method will send EOF error
func (a *Slurm) TailFile(s api.WorkloadManager_TailFileServer) error {
	r, err := s.Recv()
	if err != nil {
		return errors.Wrap(err, "can't receive request")
	}

	fd, err := a.client.Tail(r.Path)
	if err != nil {
		return errors.Wrapf(err, "can't tail file at %s", r.Path)
	}
	defer func(p string) {
		log.Printf("Tail file at %s finished", p)
	}(r.Path)

	requestCh := make(chan *api.TailFileRequest)
	go func() {
		r, err := s.Recv()
		if err != nil {
			if err != io.EOF {
				log.Printf("can't recive request err: %s", err)
			}
			return
		}

		requestCh <- r
	}()

	buff := make([]byte, 128)

	for {
		select {
		case <-s.Context().Done():
			return s.Context().Err()
		case r := <-requestCh:
			if r.Action == api.TailAction_ReadToEndAndClose {
				_ = fd.Close()
			}
		case <-time.Tick(100 * time.Millisecond):
			n, err := fd.Read(buff)
			if err != nil && n == 0 {
				return err
			}

			if n == 0 {
				continue
			}

			if err := s.Send(&api.Chunk{Content: buff[:n]}); err != nil {
				return errors.Wrap(err, "can't send chunk")
			}
		}
	}
}

func (a *Slurm) Resources(context.Context, *api.ResourcesRequest) (*api.ResourcesResponse, error) {
	slurmResources, err := a.client.Resources(a.cfg.Partition)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get resources for partition %s", a.cfg.Partition)
	}

	response := &api.ResourcesResponse{
		Nodes:      a.cfg.Resources.Nodes,
		CpuPerNode: a.cfg.Resources.CpuPerNode,
		MemPerNode: a.cfg.Resources.MemPerNode,
		WallTime:   int64(a.cfg.Resources.WallTime.Seconds()),
	}

	for _, f := range slurmResources.Features {
		response.Features = append(response.Features, &api.Feature{
			Name:     f.Name,
			Version:  f.Version,
			Quantity: f.Quantity,
		})
	}

	for _, f := range a.cfg.Resources.AdditionalFeatures {
		response.Features = append(response.Features, &api.Feature{
			Name:     f.Name,
			Version:  f.Version,
			Quantity: f.Quantity,
		})
	}

	if a.cfg.Resources.AutoNodes || response.Nodes == 0 {
		response.Nodes = slurmResources.Nodes
	}

	if a.cfg.Resources.AutoCpuPerNode || response.CpuPerNode == 0 {
		response.CpuPerNode = slurmResources.CpuPerNode
	}

	if a.cfg.Resources.AutoMemPerNode || response.MemPerNode == 0 {
		response.MemPerNode = slurmResources.MemPerNode
	}

	if a.cfg.Resources.AutoWallTime || response.WallTime == 0 {
		response.WallTime = int64(slurmResources.WallTime.Seconds())
	}

	return response, nil
}

func mapSStepsToProtoSteps(ss []*slurm.JobStepInfo) ([]*api.JobStepInfo, error) {
	pSteps := make([]*api.JobStepInfo, len(ss))

	for i, s := range ss {
		var startedAt *timestamp.Timestamp
		if s.StartedAt != nil {
			pt, err := ptypes.TimestampProto(*s.StartedAt)
			if err != nil {
				return nil, errors.Wrap(err, "can't convert started go time to proto time")
			}

			startedAt = pt
		}

		var finishedAt *timestamp.Timestamp
		if s.FinishedAt != nil {
			pt, err := ptypes.TimestampProto(*s.FinishedAt)
			if err != nil {
				return nil, errors.Wrap(err, "can't convert finished go time to proto time")
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
				return nil, errors.Wrap(err, "can't convert submit go time to proto time")
			}

			submitTime = pt
		}

		var startTime *timestamp.Timestamp
		if inf.StartTime != nil {
			pt, err := ptypes.TimestampProto(*inf.StartTime)
			if err != nil {
				return nil, errors.Wrap(err, "can't convert start go time to proto time")
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
