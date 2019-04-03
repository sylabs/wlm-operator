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

package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/pkg/slurm"
)

const (
	slurmRunEndpointT     = "%s/srun"
	slurmBatchEndpointT   = "%s/sbatch"
	slurmSacctEndpointT   = "%s/sacct/%d"
	slurmScancelEndpointT = "%s/scancel/%d"
	slurmOpenEndpointT    = "%s/open?path=%s"
)

var (
	// ErrNot200 is returned whenever an HTTP request results in a status code other than 200.
	ErrNot200 = errors.New("not 200 code in response")
)

// Config is a Client's config that will be used for each outgoing HTTP call.
type Config struct {
	// ControllerAddress is a Slurm controller address to connect to.
	// Slurm controller is located on a Slurm submission host, in other words
	// Slurm controller is a local Slurm client that serves HTTP requests.
	ControllerAddress string
	// TimeOut is an HTTP timeout in seconds that should be respected
	// during all HTTP calls.
	TimeOut int64
}

// Client implements Slurm interface for communicating with
// a remote Slurm cluster over HTTP.
type Client struct {
	conf Config
	cl   *http.Client
}

// NewClient initializes new HTTP client that will be interacting with Slurm cluster.
func NewClient(c Config) (*Client, error) {
	return &Client{
		cl: &http.Client{
			Timeout: time.Second * time.Duration(c.TimeOut),
		},
		conf: c,
	}, nil
}

// SAcct returns information about a submitted batch job.
func (c *Client) SAcct(id int64) ([]*slurm.JobInfo, error) {
	resp, err := c.cl.Get(fmt.Sprintf(slurmSacctEndpointT, c.conf.ControllerAddress, id))
	if err != nil {
		return nil, errors.Wrap(err, "could not send sacct request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Wrap(ErrNot200, resp.Status)
	}

	var infos []*slurm.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return nil, errors.Wrap(err, "could not decode sacct response")
	}

	return infos, nil
}

// SBatch submits batch job and returns job id if succeeded.
func (c *Client) SBatch(batch string) (int64, error) {
	req := struct {
		Command string `json:"command"`
	}{Command: batch}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(req); err != nil {
		return 0, errors.Wrap(err, "could not encode request")
	}

	resp, err := c.cl.Post(fmt.Sprintf(slurmBatchEndpointT, c.conf.ControllerAddress), "application/json", &body)
	if err != nil {
		return 0, errors.Wrap(err, "could not send sbatch request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, errors.Wrap(ErrNot200, resp.Status)
	}

	idS, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, errors.Wrap(err, "could not read response body")
	}

	id, err := strconv.ParseInt(string(idS), 10, 0)
	return id, errors.Wrap(err, "could not parse job id")
}

// SCancel cancels batch job.
func (c *Client) SCancel(id int64) error {
	resp, err := c.cl.Get(fmt.Sprintf(slurmScancelEndpointT, c.conf.ControllerAddress, id))
	if err != nil {
		return errors.Wrap(err, "could not send scancel request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Wrap(ErrNot200, resp.Status)
	}

	return nil
}

// SRun runs passed command with args in Slurm cluster using context.
// Srun output is returned uninterpreted as a byte slice.
func (c *Client) SRun(ctx context.Context, command string, args ...string) ([]byte, error) {
	req := struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}{Command: command, Args: args}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(req); err != nil {
		return nil, errors.Wrap(err, "could not encode request")
	}

	resp, err := c.cl.Post(fmt.Sprintf(slurmRunEndpointT, c.conf.ControllerAddress), "application/json", &body)
	if err != nil {
		return nil, errors.Wrap(err, "could not send srun request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Wrap(ErrNot200, resp.Status)
	}

	logs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "could not read response body")
	}

	return logs, nil
}

// Open opens arbitrary file in a read-only mode on
// Slurm cluster, e.g. for collecting job results.
// It is a caller's responsibility to call Close on the returned
// file to free any allocated resources.
func (c *Client) Open(path string) (io.ReadCloser, error) {
	log.Println(path)
	resp, err := c.cl.Get(fmt.Sprintf(slurmOpenEndpointT, c.conf.ControllerAddress, path))
	if err != nil {
		return nil, errors.Wrap(err, "could not send open request")
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, slurm.ErrFileNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Wrap(ErrNot200, resp.Status)
	}

	return resp.Body, nil
}
