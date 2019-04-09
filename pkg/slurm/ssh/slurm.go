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

package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"github.com/sylabs/slurm-operator/pkg/slurm"
	"golang.org/x/crypto/ssh"
)

const (
	sacctBinaryName   = "sacct"
	sbatchBinaryName  = "sbatch"
	scancelBinaryName = "scancel"
	srunBinaryName    = "srun"
)

// Client implements Slurm interface for communicating with
// a remote Slurm cluster over ssh.
type Client struct {
	ssh *ssh.Client
}

// NewClient initializes new ssh client that will be interacting with Slurm cluster.
// Password and key are optional and depend on a specific ssh configuration.
func NewClient(user, addr, password string, key []byte) (*Client, error) {
	var auth []ssh.AuthMethod
	if key != nil {
		sig, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse private key")
		}
		auth = append(auth, ssh.PublicKeys(sig))
	}

	if password != "" {
		auth = append(auth, ssh.Password(password))
	}

	cc := &ssh.ClientConfig{
		User: user,
		Auth: auth,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	c, err := ssh.Dial("tcp", addr, cc)
	if err != nil {
		return nil, errors.Wrapf(err, "could not dial %s", addr)
	}

	return &Client{ssh: c}, nil
}

// SAcct returns information about a submitted batch job.
func (c *Client) SAcct(jobID int64) ([]*slurm.JobInfo, error) {
	s, err := c.ssh.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "could not create new ssh session")
	}

	var stderr bytes.Buffer
	s.Stderr = &stderr
	cmd := fmt.Sprintf("%s -p -n -j %d -o start,end,exitcode,state,comment,jobid,jobname", sacctBinaryName, jobID)
	out, err := s.Output(cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute sacct: %s", stderr)
	}

	jInfo, err := slurm.ParseSacctResponse(string(out))
	if err != nil {
		return nil, errors.Wrap(err, slurm.ErrInvalidSacctResponse.Error())
	}

	return jInfo, nil
}

// SBatch submits batch job and returns job id if succeeded.
func (c *Client) SBatch(command string) (int64, error) {
	s, err := c.ssh.NewSession()
	if err != nil {
		return 0, errors.Wrap(err, "could not create new ssh session")
	}

	s.Stdin = bytes.NewBufferString(command)
	cmd := fmt.Sprintf("%s --parsable", sbatchBinaryName)
	out, err := s.CombinedOutput(cmd)
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
func (c *Client) SCancel(jobID int64) error {
	s, err := c.ssh.NewSession()
	if err != nil {
		return errors.Wrap(err, "could not create new ssh session")
	}

	cmd := fmt.Sprintf("%s %s", scancelBinaryName, strconv.FormatInt(jobID, 10))
	out, err := s.CombinedOutput(cmd)
	if err != nil && out != nil {
		log.Println(string(out))
	}
	return errors.Wrap(err, "failed to execute scancel")
}

// SRun runs passed command with args in Slurm cluster using context.
// Srun output is returned uninterpreted as a byte slice.
func (c *Client) SRun(_ context.Context, command string, args ...string) ([]byte, error) {
	s, err := c.ssh.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "could not create new ssh session")
	}

	cmd := fmt.Sprintf("%s %s %s", srunBinaryName, command, strings.Join(args, " "))
	out, err := s.CombinedOutput(cmd)
	return out, errors.Wrap(err, "failed to execute srun")
}

// Open opens file on a remote host in a read-only mode.
func (c *Client) Open(path string) (io.ReadCloser, error) {
	sC, err := sftp.NewClient(c.ssh)
	if err != nil {
		return nil, errors.Wrap(err, "could not create sftp client")
	}

	file, err := sC.Open(path)
	if os.IsNotExist(err) {
		return nil, slurm.ErrFileNotFound
	}
	return file, errors.Wrapf(err, "could not open %s", path)
}
