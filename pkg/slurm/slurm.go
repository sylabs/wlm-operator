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
)

var (
	// ErrInvalidSacctResponse is returned when trying to parse sacct
	// response that is invalid.
	ErrInvalidSacctResponse = errors.New("unable to parse sacct response")
	// ErrFileNotFound is returned when Open fails to find a file.
	ErrFileNotFound = errors.New("file is not found")
)

// Client implements Slurm interface for communicating with
// a local Slurm cluster by calling Slurm binaries directly.
type Client struct{}

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

// JobInfo contains information about a Slurm job.
type JobInfo struct {
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

// JobStepInfo contains information about Slurm job step.
type JobStepInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	ExitCode   int        `json:"exit_code"`
	State      string     `json:"state"`
}

type Feature struct {
	Name     string
	Version  string
	Quantity int64
}

type Resources struct {
	Nodes      int64
	MemPerNode int64
	CpuPerNode int64
	WallTime   time.Duration
	Features   []*Feature
}

// SBatch submits batch job and returns job id if succeeded.
func (*Client) SBatch(command string) (int64, error) {
	cmd := exec.Command(sbatchBinaryName, "--parsable")
	cmd.Stdin = bytes.NewBufferString(command)

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

func (*Client) Tail(path string) (io.ReadCloser, error) {
	tr, err := tail.NewReader(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not create tail reader")
	}

	return tr, nil
}

func (*Client) SJobInfo(jobID int64) ([]*JobInfo, error) {
	cmd := exec.Command(scontrolBinaryName, "show", "jobid", strconv.FormatInt(jobID, 10))

	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get info for jobid: %d", jobID)
	}

	ji, err := JobInfoFromScontrolResponse(string(out))
	if err != nil {
		return nil, errors.Wrap(err, "can't parse scontrol response")
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

	jInfo, err := ParseSacctResponse(string(out))
	if err != nil {
		return nil, errors.Wrap(err, ErrInvalidSacctResponse.Error())
	}

	return jInfo, nil
}

// Resources returns available resources for partition
func (*Client) Resources(p string) (*Resources, error) {
	return &Resources{}, nil
}

func JobInfoFromScontrolResponse(r string) ([]*JobInfo, error) {
	r = strings.TrimSpace(r)
	rawInfos := strings.Split(r, "\n\n")

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

		ji := &JobInfo{}
		if err := ji.fillFromSlurmFields(slurmFields); err != nil {
			return nil, err
		}
		infos[i] = ji
	}

	return infos, nil
}

// ParseSacctResponse is a helper that parses sacct output and
// returns results in a convenient form.
func ParseSacctResponse(raw string) ([]*JobStepInfo, error) {
	lines := strings.Split(strings.Trim(raw, "\n"), "\n")
	infos := make([]*JobStepInfo, len(lines))
	for i, l := range lines {
		splitted := strings.Split(l, "|")
		if len(splitted) != 7 {
			return nil, errors.New("output must contain 6 sections")
		}

		startedAt, err := parseTime(splitted[0])
		if err != nil {
			return nil, err
		}

		finishedAt, err := parseTime(splitted[1])
		if err != nil {
			return nil, err
		}

		exitCodeSplitted := strings.Split(splitted[2], ":")
		if len(exitCodeSplitted) != 2 {
			return nil, errors.New("exit code must contain 2 sections")
		}
		exitCode, err := strconv.Atoi(exitCodeSplitted[0])
		if err != nil {
			return nil, err
		}
		j := JobStepInfo{
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			ExitCode:   exitCode,
			State:      splitted[3],
			ID:         splitted[4],
			Name:       splitted[5],
		}
		infos[i] = &j
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
		case "SubmitTime", "StartTime":
			t, err := parseTime(sField)
			if err != nil {
				return errors.Wrapf(err, "can't parse time: %s", sField)
			}
			val = reflect.ValueOf(t)
		case "RunTime", "TimeLimit":
			d, err := ParseDuration(sField)
			if err != nil {
				return errors.Wrapf(err, "can't parse duration: %s", sField)
			}
			val = reflect.ValueOf(d)
		default:
			val = reflect.ValueOf(sField)
		}

		reflect.ValueOf(ji).Elem().Field(i).Set(val)
	}

	return nil
}

// ParseDuration parses slurm duration string. Possible formats are:
// minutes, minutes:seconds, hours:minutes:seconds, days-hours, days-hours:minutes or days-hours:minutes:seconds
func ParseDuration(duration string) (*time.Duration, error) {
	const unlimited = "UNLIMITED"
	if duration == unlimited || duration == "" {
		return nil, nil
	}

	var err error
	var d time.Duration
	var days, hours, minutes, seconds int64
	parts := strings.Split(duration, ":")
	if len(parts) > 3 {
		return nil, errors.New("invalid duration format")
	}
	i := strings.IndexByte(parts[0], '-')
	if i != -1 {
		days, err = strconv.ParseInt(parts[0][:i], 10, 0)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid amount of days")
		}
		hours, err = strconv.ParseInt(parts[0][i+1:], 10, 0)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid amount of hours")
		}
		if len(parts) > 1 {
			minutes, err = strconv.ParseInt(parts[1], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of minutes")
			}
		}
		if len(parts) > 2 {
			seconds, err = strconv.ParseInt(parts[2], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of seconds")
			}
		}
	} else {
		switch len(parts) {
		case 1:
			minutes, err = strconv.ParseInt(parts[0], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of minutes")
			}
		case 2:
			minutes, err = strconv.ParseInt(parts[0], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of minutes")
			}
			seconds, err = strconv.ParseInt(parts[1], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of seconds")
			}
		case 3:
			hours, err = strconv.ParseInt(parts[0], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of hours")
			}
			minutes, err = strconv.ParseInt(parts[1], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of minutes")
			}
			seconds, err = strconv.ParseInt(parts[2], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of seconds")
			}
		}
	}

	d += time.Hour * 24 * time.Duration(days)
	d += time.Hour * time.Duration(hours)
	d += time.Minute * time.Duration(minutes)
	d += time.Second * time.Duration(seconds)
	return &d, nil
}

func parseTime(timeStr string) (*time.Time, error) {
	const slurmTimeLayout = "2006-01-02T15:04:05"

	if timeStr == "" || strings.ToLower(timeStr) == "unknown" {
		return nil, nil
	}

	t, err := time.Parse(slurmTimeLayout, timeStr)
	if err != nil {
		return nil, err
	}

	return &t, nil
}
