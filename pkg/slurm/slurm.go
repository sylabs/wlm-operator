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
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"

	"fmt"
)

const (
	// JobStatusCompleted means Slurm job is finished successfully.
	JobStatusCompleted = "COMPLETED"
	// JobStatusCanceled means Slurm job was cancelled.
	JobStatusCanceled = "CANCELLED"
	// JobStatusFailed means job is failed to execute successfully.
	JobStatusFailed = "FAILED"
)

var (
	// ErrInvalidSacctResponse is returned when trying to parse sacct
	// response that is invalid.
	ErrInvalidSacctResponse = errors.New("unable to parse sacct response")

	// ErrFileNotFound is returned when Open fails to find a file.
	ErrFileNotFound = errors.New("file is not found")
)

// JobInfo contains information about a single Slurm job.
type JobInfo struct {
	ID         string         `json:"id" slurm:"JobId"`
	UserID     string         `json:"user_id" slurm:"UserId"`
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

// Slurm defines interface for interacting with Slurm cluster.
// Interaction can be done in different ways, e.g over ssh, http,
// or even by calling binaries directly.
type Slurm interface {
	// SBatch submits batch job and returns job id if succeeded.
	SBatch(command string) (int64, error)
	// SCancel cancels batch job.
	SCancel(jobID int64) error
	// Open opens arbitrary file in a read-only mode on
	// Slurm cluster, e.g. for collecting job results.
	// It is a caller's responsibility to call Close on the returned
	// file to free any allocated resources. Is a file is not found
	// Open will return ErrFileNotFound.

	SJobInfo(jobID int64) (*JobInfo, error)

	Open(path string) (io.ReadCloser, error)
}

func JobInfoFromScontrolResponse(r string) (*JobInfo, error) {
	rFields := strings.Fields(r)
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
	t := reflect.TypeOf(ji)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if tagV, ok := f.Tag.Lookup("slurm"); ok {
			if sField, ok := slurmFields[tagV]; ok {
				var val reflect.Value
				switch tagV {
				case "SubmitTime", "StartTime":
					t, err := parseTime(sField)
					if err != nil {
						return nil, errors.Wrapf(err, "can't parse time: %s", sField)
					}
					val = reflect.ValueOf(t)
				case "RunTime", "TimeLimit":
					d, err := parseDuration(sField)
					if err != nil {
						return nil, errors.Wrapf(err, "can't parse duration: %s", sField)
					}
					val = reflect.ValueOf(d)
				default:
					val = reflect.ValueOf(sField)
				}

				reflect.ValueOf(&ji).Elem().Field(i).Set(val)
			}
		}
	}

	return &ji, nil
}

func parseDuration(durationStr string) (*time.Duration, error) {
	sp := strings.Split(durationStr, ":")
	if len(sp) < 3 {
		// we can skip since data is invalid or not available for that field
		return nil, nil
	}
	d, err := time.ParseDuration(fmt.Sprintf("%sh%sm%ss", sp[0], sp[1], sp[2]))
	return &d, err
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
