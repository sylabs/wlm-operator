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

	"github.com/sylabs/slurm-operator/pkg/slurm/local"
	"github.com/sylabs/slurm-operator/pkg/workload/api"
)

// Slurm implements WorkloadManagerServer
type Slurm struct {
	client *local.Client
}

// NewSlurmAPI creates a new instance of Slurm
func NewSlurmAPI(c *local.Client) *Slurm {
	return &Slurm{client: c}
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

func mapSStepsToProtoSteps(ss []*local.JobStepInfo) ([]*api.JobStepInfo, error) {
	var pSteps []*api.JobStepInfo

	for _, s := range ss {
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

			startedAt = pt
		}

		status, ok := api.JobStatus_value[s.State]
		if !ok {
			status = int32(api.JobStatus_UNKNOWN)
		}

		pSteps = append(pSteps, &api.JobStepInfo{
			Id:        s.ID,
			Name:      s.Name,
			ExitCode:  int32(s.ExitCode),
			Status:    api.JobStatus(status),
			StartTime: startedAt,
			EndTime:   finishedAt,
		})
	}

	return pSteps, nil
}

func mapSInfoToProtoInfo(si *local.JobInfo) (*api.JobInfo, error) {
	var submitTime *timestamp.Timestamp
	if si.SubmitTime != nil {
		pt, err := ptypes.TimestampProto(*si.SubmitTime)
		if err != nil {
			return nil, errors.Wrap(err, "can't convert submit go time to proto time")
		}

		submitTime = pt
	}

	var startTime *timestamp.Timestamp
	if si.StartTime != nil {
		pt, err := ptypes.TimestampProto(*si.StartTime)
		if err != nil {
			return nil, errors.Wrap(err, "can't convert start go time to proto time")
		}

		startTime = pt
	}

	var runTime *duration.Duration
	if si.RunTime != nil {
		runTime = ptypes.DurationProto(*si.RunTime)
	}

	var timeLimit *duration.Duration
	if si.TimeLimit != nil {
		timeLimit = ptypes.DurationProto(*si.TimeLimit)
	}

	status, ok := api.JobStatus_value[si.State]
	if !ok {
		status = int32(api.JobStatus_UNKNOWN)
	}

	pi := api.JobInfo{
		Id:         si.ID,
		UserId:     si.UserID,
		Name:       si.Name,
		ExitCode:   si.ExitCode,
		Status:     api.JobStatus(status),
		SubmitTime: submitTime,
		StartTime:  startTime,
		RunTime:    runTime,
		TimeLimit:  timeLimit,
		WorkingDir: si.WorkDir,
		StdOut:     si.StdOut,
		StdErr:     si.StdErr,
		Partition:  si.Partition,
		NodeList:   si.NodeList,
		BatchHost:  si.BatchHost,
		NumNodes:   si.NumNodes,
	}

	return &pi, nil
}
