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

package slurm

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/pkg/tail"
)

const (
	sbatchBinaryName   = "sbatch"
	scancelBinaryName  = "scancel"
	scontrolBinaryName = "scontrol"
	sacctBinaryName    = "sacct"

	submitTime = "SubmitTime"
	startTime  = "StartTime"
	runTime    = "RunTime"
	timeLimit  = "TimeLimit"
)

var (
	// ErrDurationIsUnlimited means that duration field has value UNLIMITED
	ErrDurationIsUnlimited = errors.New("duration is unlimited")

	// ErrInvalidSacctResponse is returned when trying to parse sacct
	// response that is invalid.
	ErrInvalidSacctResponse = errors.New("unable to parse sacct response")

	// ErrFileNotFound is returned when Open fails to find a file.
	ErrFileNotFound = errors.New("file is not found")
)

type (
	// Client implements Slurm interface for communicating with
	// a local Slurm cluster by calling Slurm binaries directly.
	Client struct{}

	// JobInfo contains information about a Slurm job.
	JobInfo struct {
		ID         string         `json:"id" slurm:"JobId"`
		UserID     string         `json:"user_id" slurm:"UserId"`
		ArrayJobID string         `json:"array_job_id" slurm:"ArrayJobId"`
		Name       string         `json:"name" slurm:"JobName"`
		ExitCode   string         `json:"exit_code" slurm:"ExitCode"`
		State      string         `json:"state" slurm:"JobState"`
		SubmitTime *time.Time     `json:"submit_time" slurm:"SubmitTime"`
		StartTime  *time.Time     `json:"start_time" slurm:"StartTime"`
		RunTime    *time.Duration `json:"run_time" slurm:"RunTime"`
		TimeLimit  *time.Duration `json:"time_limit" slurm:"TimeLimit"`
		WorkDir    string         `json:"work_dir" slurm:"WorkDir"`
		StdOut     string         `json:"std_out" slurm:"StdOut"`
		StdErr     string         `json:"std_err" slurm:"StdErr"`
		Partition  string         `json:"partition" slurm:"Partition"`
		NodeList   string         `json:"node_list" slurm:"NodeList"`
		BatchHost  string         `json:"batch_host" slurm:"BatchHost"`
		NumNodes   string         `json:"num_nodes" slurm:"NumNodes"`
	}

	// JobStepInfo contains information about a single Slurm job step.
	JobStepInfo struct {
		ID         string     `json:"id"`
		Name       string     `json:"name"`
		StartedAt  *time.Time `json:"started_at"`
		FinishedAt *time.Time `json:"finished_at"`
		ExitCode   int        `json:"exit_code"`
		State      string     `json:"state"`
	}

	// Feature represents a single feature enabled on a Slurm partition.
	// TODO use it.
	Feature struct {
		Name     string
		Version  string
		Quantity int64
	}

	// Resources contain a list of available resources on a Slurm partition.
	Resources struct {
		Nodes      int64
		MemPerNode int64
		CPUPerNode int64
		WallTime   time.Duration
		Features   []Feature
	}
)

// NewClient returns new local client.
func NewClient() (*Client, error) {
	var missing []string
	for _, bin := range []string{sacctBinaryName, sbatchBinaryName, scancelBinaryName, scontrolBinaryName} {
		_, err := exec.LookPath(bin)
		if err != nil {
			missing = append(missing, bin)
		}
	}
	if len(missing) != 0 {
		return nil, errors.Errorf("no slurm binaries found: %s", strings.Join(missing, ", "))
	}
	return &Client{}, nil
}

// SBatch submits batch job and returns job id if succeeded.
func (*Client) SBatch(script, partition string) (int64, error) {
	var partitionOpt string
	if partition != "" {
		partitionOpt = "--partition=" + partition
	}
	cmd := exec.Command(sbatchBinaryName, "--parsable", partitionOpt)
	cmd.Stdin = bytes.NewBufferString(script)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if out != nil {
			log.Println(string(out))
		}
		return 0, errors.Wrap(err, "failed to execute sbatch")
	}

	id, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, errors.Wrap(err, "could not parse job id")
	}

	return int64(id), nil
}

// SCancel cancels batch job.
func (*Client) SCancel(jobID int64) error {
	cmd := exec.Command(scancelBinaryName, strconv.FormatInt(jobID, 10))

	out, err := cmd.CombinedOutput()
	if err != nil && out != nil {
		log.Println(string(out))
	}
	return errors.Wrap(err, "failed to execute scancel")
}

// Open opens arbitrary file at path in a read-only mode.
func (*Client) Open(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}
	return file, errors.Wrapf(err, "could not open %s", path)
}

// Tail opens arbitrary file at path in a read-only mode.
// Unlike Open, Tail will watch file changes in a real-time.
func (*Client) Tail(path string) (io.ReadCloser, error) {
	tr, err := tail.NewReader(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not create tail reader")
	}

	return tr, nil
}

// SJobInfo returns information about a particular slurm job by ID.
func (*Client) SJobInfo(jobID int64) ([]*JobInfo, error) {
	cmd := exec.Command(scontrolBinaryName, "show", "jobid", strconv.FormatInt(jobID, 10))

	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get info for jobid: %d", jobID)
	}

	ji, err := jobInfoFromScontrolResponse(string(out))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse scontrol response")
	}

	return ji, nil
}

// SJobSteps returns information about a submitted batch job.
func (*Client) SJobSteps(jobID int64) ([]*JobStepInfo, error) {
	cmd := exec.Command(sacctBinaryName,
		"-p",
		"-n",
		"-j",
		strconv.FormatInt(jobID, 10),
		"-o start,end,exitcode,state,jobid,jobname",
	)

	out, err := cmd.Output()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if ok {
			return nil, errors.Wrapf(err, "failed to execute sacct: %s", ee.Stderr)
		}
		return nil, errors.Wrap(err, "failed to execute sacct")
	}

	jInfo, err := parseSacctResponse(string(out))
	if err != nil {
		return nil, errors.Wrap(err, ErrInvalidSacctResponse.Error())
	}

	return jInfo, nil
}

// Resources returns available resources for a partition.
func (*Client) Resources(partition string) (*Resources, error) {
	cmd := exec.Command(scontrolBinaryName, "show", "partition", partition)
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "could not get partition info")
	}

	r, err := parseResources(string(out))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse partition resources")
	}

	return r, nil
}

func jobInfoFromScontrolResponse(jobInfo string) ([]*JobInfo, error) {
	jobInfo = strings.TrimSpace(jobInfo)
	rawInfos := strings.Split(jobInfo, "\n\n")

	infos := make([]*JobInfo, len(rawInfos))
	for i, raw := range rawInfos {
		rFields := strings.Fields(raw)
		slurmFields := make(map[string]string)
		for _, f := range rFields {
			s := strings.Split(f, "=")
			if len(s) != 2 {
				// just skipping empty fields
				continue
			}
			slurmFields[s[0]] = s[1]
		}

		var ji JobInfo
		if err := ji.fillFromSlurmFields(slurmFields); err != nil {
			return nil, err
		}
		infos[i] = &ji
	}
	return infos, nil
}

func (ji *JobInfo) fillFromSlurmFields(fields map[string]string) error {
	t := reflect.TypeOf(*ji)
	for i := 0; i < t.NumField(); i++ {
		tagV, ok := t.Field(i).Tag.Lookup("slurm")
		if !ok {
			continue
		}

		sField, ok := fields[tagV]
		if !ok {
			continue
		}

		var val reflect.Value
		switch tagV {
		case submitTime, startTime:
			t, err := parseTime(sField)
			if err != nil {
				return errors.Wrapf(err, "could not parse time: %s", sField)
			}
			val = reflect.ValueOf(t)
		case runTime, timeLimit:
			d, err := ParseDuration(sField)
			if err != nil {
				if err == ErrDurationIsUnlimited {
					continue
				}

				return errors.Wrapf(err, "could not parse duration: %s", sField)
			}
			val = reflect.ValueOf(d)
		default:
			val = reflect.ValueOf(sField)
		}

		reflect.ValueOf(ji).Elem().Field(i).Set(val)
	}

	return nil
}
