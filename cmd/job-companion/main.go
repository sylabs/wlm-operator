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
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/pkg/slurm"
	"github.com/sylabs/slurm-operator/pkg/slurm/rest"
)

const envJobName = "JOB_NAME"

var (
	jobName = os.Getenv(envJobName)

	sock      = flag.String("socket", "/red-box.sock", "unix socket to connect to red-box")
	mountPath = flag.String("cr-mount", "",
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

	log.Printf("Job will be executed locally by red-box at: %s", *sock)
	client, err := getLocalClient(*sock)
	if err != nil {
		log.Fatal(err)
	}

	var ops *collectOptions
	if mp := *mountPath; mp != "" {
		ops = &collectOptions{
			Mount: mp,
			From:  *resultsPath,
		}
	}

	if err := runBatch(client, *batch, ops); err != nil {
		log.Fatalf("could not run batch: %v", err)
	}

	log.Println("Job finished")
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
			sInfo, err := c.SJobInfo(id)
			if err != nil {
				return err
			}
			state := sInfo.State // job info contains several steps. First step shows if job execution succeeded
			if state == slurm.JobStatusCompleted ||
				state == slurm.JobStatusFailed ||
				state == slurm.JobStatusCanceled {

				log.Printf("JobID:%s State:%s ExitCode:%s Name:%s",
					sInfo.ID,
					sInfo.State,
					sInfo.ExitCode,
					sInfo.Name,
				)

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

	toFile, err := os.Create(path.Join(dirName, filepath.Base(cOps.From)))
	if err != nil {
		return errors.Wrap(err, "could not create file with results on mounted volume")
	}

	if _, err := io.Copy(toFile, fromFile); err != nil {
		return errors.Wrap(err, "can't copy from results file to mounted volume file")
	}

	return nil
}
