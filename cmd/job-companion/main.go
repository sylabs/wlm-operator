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

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	rd "github.com/sylabs/slurm-operator/internal/resource-daemon"
	"github.com/sylabs/slurm-operator/pkg/slurm"
	"github.com/sylabs/slurm-operator/pkg/slurm/rest"
	"github.com/sylabs/slurm-operator/pkg/slurm/ssh"
	"gopkg.in/yaml.v2"
)

const envJobName = "JOB_NAME"

var (
	jobName = os.Getenv(envJobName)

	configPath = flag.String("config", "", "slurm config path on host machine")
	overSSH    = flag.Bool("ssh", false, "defines whether job will be executed over ssh or locally")
	mountPath  = flag.String("cr-mount", "",
		"path to the volume/directory where results should be collected, empty if results should not be collected")
	resultsPath = flag.String("file-to-collect", "",
		"path to a specific file that should be collected as job result, if omitted - default slurm-{JobID}.out will be collected")
	batch = flag.String("batch", "", "batch script that will be executed on slurm cluster")
)

type collectOptions struct {
	Mount string
	From  string
}

func main() {
	flag.Parse()

	if *batch == "" {
		log.Fatal("batch should be provided")
	}

	if *configPath == "" {
		log.Fatal("config path cannot be empty")
	}

	// reading slurm config on host machine
	f, err := os.Open(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	var cfg rd.NodeConfig
	err = yaml.NewDecoder(f).Decode(&cfg)
	_ = f.Close()
	if err != nil {
		log.Fatal(err)
	}

	var client slurm.Slurm
	if *overSSH {
		log.Printf("Job will be executed over SSH at: %s", cfg.SSHAddr)
		client, err = getSSHClient(cfg.SSHAddr)
		if err != nil {
			log.Fatal(err)
		}

	} else {
		log.Printf("Job will be executed locally by slurm-controller at: %s", cfg.LocalAddr)
		client, err = getLocalClient(cfg.LocalAddr)
		if err != nil {
			log.Fatal(err)
		}
	}

	var ops *collectOptions
	if mp := *mountPath; mp != "" {
		ops = &collectOptions{
			Mount: mp,
			From:  *resultsPath,
		}
	}

	if err := runBatch(client, *batch, ops); err != nil {
		log.Fatal("running batch ", err)
	}

	log.Println("Job finished")
}

func getSSHClient(addr string) (*ssh.Client, error) {
	const (
		envSSHPass = "SSH_PASSWORD"
		envSSHKey  = "SSH_KEY"
		envSSHUser = "SSH_USER"
	)

	sshPass := os.Getenv(envSSHPass)
	sshKey := os.Getenv(envSSHKey)
	sshUser := os.Getenv(envSSHUser)

	var key []byte
	if sshKey != "" {
		key = []byte(sshKey)
	}

	client, err := ssh.NewClient(sshUser, addr, sshPass, key)
	if err != nil {
		return nil, errors.Wrap(err, "initializing ssh client")
	}

	return client, nil
}

func getLocalClient(addr string) (*rest.Client, error) {
	c := rest.Config{
		ControllerAddress: addr,
		TimeOut:           10,
	}
	client, err := rest.NewClient(c)
	if err != nil {
		return nil, errors.Wrap(err, "initializing rest client")
	}

	return client, nil
}

func runBatch(c slurm.Slurm, batch string, cOps *collectOptions) error {
	id, err := c.SBatch(batch)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			sInfo, err := c.SAcct(id)
			if err != nil {
				return err
			}
			if len(sInfo) == 0 {
				return errors.New("unexpected response from sacct")
			}
			state := sInfo[0].State // job info contains several steps. First step shows if job execution succeeded
			if state == slurm.JobStatusCompleted ||
				state == slurm.JobStatusFailed ||
				state == slurm.JobStatusCanceled {
				printSInfo(sInfo)

				switch state {
				case slurm.JobStatusFailed:
					return errors.New("job failed")
				case slurm.JobStatusCanceled:
					return errors.New("job canceled")
				case slurm.JobStatusCompleted:
					if cOps != nil {
						return collectResults(c, id, cOps)
					}
					return nil
				}
			}
		}
	}
}

func collectResults(c slurm.Slurm, jobID int64, cOps *collectOptions) error {
	if cOps.From == "" {
		// in case from is not specified we are using default slurm-{jobID}.out template
		cOps.From = fmt.Sprintf("slurm-%d.out", jobID)
	}

	fromFile, err := c.Open(cOps.From)
	if err != nil {
		return errors.Wrapf(err, "can't open file with results on remote host file name: %s", cOps.From)
	}
	defer fromFile.Close()

	// creating folder with JOB_NAME on attached volume
	dirName := path.Join(cOps.Mount, jobName)
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return errors.Wrap(err, "can't create dir on mounted volume")
	}

	toFile, err := os.Create(path.Join(dirName, cOps.From))
	if err != nil {
		return errors.Wrap(err, "can't create file with results on mounted volume")
	}

	if _, err := io.Copy(toFile, fromFile); err != nil {
		return errors.Wrap(err, "can't copy from results file to mounted volume file")
	}

	return nil
}

func printSInfo(infos []*slurm.JobInfo) {
	for _, i := range infos {
		log.Printf("JobID:%s State:%s ExitCode:%d Name:%s", i.ID, i.State, i.ExitCode, i.Name)
	}
}
