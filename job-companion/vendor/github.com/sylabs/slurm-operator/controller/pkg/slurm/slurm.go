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
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
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
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	ExitCode   int       `json:"exit_code"`
	State      string    `json:"state"`
	Comment    string    `json:"comment"`
}

// Slurm defines interface for interacting with Slurm cluster.
// Interaction can be done in different ways, e.g over ssh, http,
// or even by calling binaries directly.
type Slurm interface {
	// SAcct returns information about a submitted batch job.
	SAcct(jobID int64) ([]*JobInfo, error)
	// SBatch submits batch job and returns job id if succeeded.
	SBatch(command string) (int64, error)
	// SCancel cancels batch job.
	SCancel(jobID int64) error
	// SRun runs passed command with args in Slurm cluster using context.
	// Srun output is returned uninterpreted as a byte slice.
	SRun(ctx context.Context, command string, args ...string) ([]byte, error)
	// Open opens arbitrary file in a read-only mode on
	// Slurm cluster, e.g. for collecting job results.
	// It is a caller's responsibility to call Close on the returned
	// file to free any allocated resources. Is a file is not found
	// Open will return ErrFileNotFound.
	Open(path string) (io.ReadCloser, error)
}

// ParseSacctResponse is a helper that parses sacct output and
// returns results in a convenient form.
func ParseSacctResponse(raw string) ([]*JobInfo, error) {
	lines := strings.Split(strings.Trim(raw, "\n"), "\n")
	infos := make([]*JobInfo, len(lines))
	for i, l := range lines {
		splitted := strings.Split(l, "|")
		if len(splitted) != 8 {
			return nil, errors.New("have to be 7 sections")
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
			return nil, errors.New("exit code have to contain 2 sections")
		}
		exitCode, err := strconv.Atoi(exitCodeSplitted[0])
		if err != nil {
			return nil, err
		}
		j := JobInfo{
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			ExitCode:   exitCode,
			State:      splitted[3],
			Comment:    splitted[4],
			ID:         splitted[5],
			Name:       splitted[6],
		}
		infos[i] = &j
	}

	return infos, nil
}

func parseTime(t string) (time.Time, error) {
	if t == "" || strings.ToLower(t) == "unknown" {
		return time.Time{}, nil
	}

	return time.Parse(time.RFC3339, t+"Z")
}
