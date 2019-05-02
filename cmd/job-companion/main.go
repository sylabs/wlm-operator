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
	"context"
	"flag"
	"io"
	"io/ioutil"
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
		"path to a specific file that should be collected as job result")
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
		if *resultsPath == "" {
			log.Fatal("file-to-collect can't be empty when cr-mount is specified")
		}

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
	sInfo, err := c.SJobInfo(id)
	if err != nil {
		return err
	}

	log.Printf("JobID: %d", id)

	ctx, cancelTailLogs := context.WithCancel(context.Background())
	tailLogsDone := tailLogs(ctx, c, sInfo.StdOut)

	for {
		time.Sleep(1 * time.Second)

		sInfo, err = c.SJobInfo(id)
		if err != nil {
			cancelTailLogs()
			return err
		}

		state := sInfo.State
		if state == slurm.JobStatusCompleted ||
			state == slurm.JobStatusFailed ||
			state == slurm.JobStatusCanceled {

			time.Sleep(10 * time.Second) // TODO remome after migration to grpc. Give some time to finish pulling job logs
			cancelTailLogs()
			<-tailLogsDone // need to wail till all logs will be printed, not to ruin formatting

			switch state {
			case slurm.JobStatusFailed:
				// in other way logs are already printed
				if sInfo.StdOut != sInfo.StdErr {
					if err := logErrOutput(c, sInfo.StdErr); err != nil {
						log.Printf("Can't print error logs err: %s", err)
					}
				}

				if err := logJobSteps(c, id); err != nil {
					log.Printf("Can't print steps info err: %s", err)
				}

				return errors.New("job failed")
			case slurm.JobStatusCanceled:
				if err := logJobSteps(c, id); err != nil {
					log.Printf("Can't print steps info err: %s", err)
				}

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

func logErrOutput(c slurm.Slurm, path string) error {
	f, err := c.Open(path)
	if err != nil {
		return err
	}

	logs, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	log.Printf("Stderr output from %s", path)
	log.Println(string(logs))
	return nil
}

func logJobSteps(c slurm.Slurm, id int64) error {
	steps, err := c.SJobSteps(id)
	if err != nil {
		return err
	}

	for _, i := range steps {
		log.Printf("JobID:%s State:%s ExitCode:%d Name:%s", i.ID, i.State, i.ExitCode, i.Name)
	}
	return nil
}

func tailLogs(ctx context.Context, c slurm.Slurm, logFile string) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		f, err := c.Tail(logFile)
		if err != nil {
			log.Printf("Can't tail file %s err: %s", logFile, err)
			return
		}
		defer f.Close()

		buffCh := make(chan []byte)

		// since reading from f is blocking we need to do it in a separate gorutine
		go func() {
			for {
				buff := make([]byte, 128)
				n, err := f.Read(buff)
				if err != nil {
					return
				}

				buffCh <- buff[:n]
			}
		}()

		for {
			select {
			case <-ctx.Done():
				log.Println("Tail logs finished from context")
				return
			case chunk := <-buffCh:
				os.Stdout.Write(chunk)
			}
		}
	}()

	return done
}

func collectResults(c slurm.Slurm, jobID int64, cOps *collectOptions) error {
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
