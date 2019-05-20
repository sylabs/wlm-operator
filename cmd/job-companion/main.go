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
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/sylabs/slurm-operator/pkg/workload/api"

	"google.golang.org/grpc"
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

	conn, err := grpc.Dial("unix://"+*sock, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("can't connect to %s %s", *sock, err)
	}
	client := api.NewWorkloadManagerClient(conn)

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

func runBatch(c api.WorkloadManagerClient, batch string, cOps *collectOptions) error {
	sjResp, err := c.SubmitJob(context.Background(), &api.SubmitJobRequest{Script: batch})
	if err != nil {
		return err
	}

	jobID := sjResp.JobId

	infoResp, err := c.JobInfo(context.Background(), &api.JobInfoRequest{JobId: jobID})
	if err != nil {
		return err
	}
	info := infoResp.Info[0] // response is always contains at leas one element

	log.Printf("JobID: %d", jobID)

	ctx, cancelTailLogs := context.WithCancel(context.Background())
	var tailLogsDone chan struct{}
	// we are not tailing logs for JobArrays
	// since there is no a correct solution how we can print multiple parallel running job outputs
	// without affecting each other
	if info.ArrayId == "" {
		tailLogsDone = tailLogs(ctx, c, info.StdOut)
	}

	stopLogs := func() {
		cancelTailLogs()

		// if tail logs done is nil that means that job is a JobArray
		// and we are not tailing logs for such jobs
		if tailLogsDone == nil {
			return
		}

		<-tailLogsDone // need to wail till all logs will be printed, not to ruin formatting
	}

	for {
		time.Sleep(1 * time.Second)

		infoResp, err = c.JobInfo(context.Background(), &api.JobInfoRequest{JobId: jobID})
		if err != nil {
			// waits till logs read reaches EOF
			stopLogs()
			return err
		}
		info = infoResp.Info[0]

		state := info.Status
		if state == api.JobStatus_COMPLETED ||
			state == api.JobStatus_FAILED ||
			state == api.JobStatus_CANCELLED {

			// waits till logs read reaches EOF
			stopLogs()

			switch state {
			case api.JobStatus_FAILED:
				// in other way logs are already printed
				if info.StdOut != info.StdErr {
					if err := logErrOutput(c, info.StdErr); err != nil {
						log.Printf("Can't print error logs err: %s", err)
					}
				}

				if err := logJobSteps(c, jobID); err != nil {
					log.Printf("Can't print steps info err: %s", err)
				}

				return errors.New("job failed")
			case api.JobStatus_CANCELLED:
				if err := logJobSteps(c, jobID); err != nil {
					log.Printf("Can't print steps info err: %s", err)
				}

				return errors.New("job canceled")
			case api.JobStatus_COMPLETED:
				if cOps != nil {
					return collectResults(c, cOps)
				}
				return nil
			}
		}
	}
}

func logErrOutput(c api.WorkloadManagerClient, path string) error {
	f, err := c.OpenFile(context.Background(), &api.OpenFileRequest{Path: path})
	if err != nil {
		return err
	}
	defer f.CloseSend()

	log.Printf("Stderr output from %s", path)
	for {
		chunk, err := f.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return errors.Wrap(err, "can't receive chunk")
		}

		log.Print(string(chunk.Content))
	}
}

func logJobSteps(c api.WorkloadManagerClient, id int64) error {
	stepsResp, err := c.JobSteps(context.Background(), &api.JobStepsRequest{JobId: id})
	if err != nil {
		return err
	}

	for _, i := range stepsResp.JobSteps {
		log.Printf("JobID:%s State:%s ExitCode:%d Name:%s",
			i.Id,
			api.JobStatus_name[int32(i.Status)],
			i.ExitCode,
			i.Name,
		)
	}
	return nil
}

func tailLogs(ctx context.Context, c api.WorkloadManagerClient, logFile string) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)
		tf, err := c.TailFile(context.Background())
		if err != nil {
			if err != io.EOF {
				log.Printf("Can't tail file %s err: %s", logFile, err)
			}
			return
		}
		defer tf.CloseSend()
		if err := tf.Send(&api.TailFileRequest{Path: logFile, Action: api.TailAction_Start}); err != nil {
			log.Printf("Can't send tail request err: %s", err)
		}

		buffCh := make(chan []byte)

		// since reading from tf is blocking we need to do it in a separate gorutine
		go func() {
			defer close(buffCh)
			for {
				chunk, err := tf.Recv()
				if err != nil {
					return
				}

				buffCh <- chunk.Content
			}
		}()

		done := ctx.Done()

		for {
			select {
			case <-done:
				done = nil
				if err := tf.Send(&api.TailFileRequest{Path: logFile, Action: api.TailAction_ReadToEndAndClose}); err != nil {
					log.Printf("Can't send tail request err: %s", err)
					return
				}
			case chunk, ok := <-buffCh:
				if !ok {
					return
				}

				_, err = os.Stdout.Write(chunk)
				if err != nil {
					log.Printf("Can't write to stdout err: %s", err)
					return
				}
			}
		}
	}()

	return done
}

func collectResults(c api.WorkloadManagerClient, cOps *collectOptions) error {
	fromFile, err := c.OpenFile(context.Background(), &api.OpenFileRequest{Path: cOps.From})
	if err != nil {
		return errors.Wrapf(err, "can't open file with results on remote host file name: %s", cOps.From)
	}
	defer fromFile.CloseSend()

	// creating folder with JOB_NAME on attached volume
	dirName := path.Join(cOps.Mount, jobName)
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return errors.Wrap(err, "can't create dir on mounted volume")
	}

	toFile, err := os.Create(path.Join(dirName, filepath.Base(cOps.From)))
	if err != nil {
		return errors.Wrap(err, "could not create file with results on mounted volume")
	}
	defer toFile.Close()

	for {
		chunk, err := fromFile.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return errors.Wrap(err, "can't receive chunk")
		}

		if _, err := toFile.Write(chunk.Content); err != nil {
			return errors.Wrap(err, "can't write to file")
		}
	}
}
