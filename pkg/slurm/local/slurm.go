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

package local

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/pkg/slurm"
)

const (
	sacctBinaryName   = "sacct"
	sbatchBinaryName  = "sbatch"
	scancelBinaryName = "scancel"
	srunBinaryName    = "srun"
)

// Client implements Slurm interface for communicating with
// a local Slurm cluster by calling Slurm binaries directly.
type Client struct{}

// NewClient returns new local client.
func NewClient() (*Client, error) {
	return &Client{}, nil
}

// SAcct returns information about a submitted batch job.
func (*Client) SAcct(jobID int64) ([]*slurm.JobInfo, error) {
	cmd := exec.Command(sacctBinaryName,
		"-p",
		"-n",
		"-j",
		strconv.FormatInt(jobID, 10),
		"-o start,end,exitcode,state,comment,jobid,jobname",
	)

	out, err := cmd.Output()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if ok {
			return nil, errors.Wrapf(err, "failed to execute sacct: %s", ee.Stderr)
		}
		return nil, errors.Wrap(err, "failed to execute sacct")
	}

	jInfo, err := slurm.ParseSacctResponse(string(out))
	if err != nil {
		return nil, errors.Wrap(err, slurm.ErrInvalidSacctResponse.Error())
	}

	return jInfo, nil
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

// SRun runs passed command with args in Slurm cluster using context.
// Srun output is returned uninterpreted as a byte slice.
func (*Client) SRun(ctx context.Context, command string, args ...string) ([]byte, error) {
	commandWithParams := []string{command}
	commandWithParams = append(commandWithParams, args...)

	cmd := exec.CommandContext(ctx, srunBinaryName, commandWithParams...)
	out, err := cmd.CombinedOutput()
	return out, errors.Wrap(err, "failed to execute srun")
}

// Open opens arbitrary file at path in a read-only mode.
func (*Client) Open(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, slurm.ErrFileNotFound
	}
	return file, errors.Wrapf(err, "could not open %s", path)
}
