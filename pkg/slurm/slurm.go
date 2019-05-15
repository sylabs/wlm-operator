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

// #cgo CFLAGS: -I${SRCDIR}/include
// #cgo LDFLAGS: -L/usr/lib/x86_64-linux-gnu/slurm/ -lslurm
// #include "slurm.h" // slurm api calls
// #include "slurm_errno.h" // slurm error definitions
import "C"

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/pkg/tail"
)

const (
	sbatchBinaryName  = "sbatch"
	scancelBinaryName = "scancel"
	sacctBinaryName   = "sacct"
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
	for _, bin := range []string{sacctBinaryName, sbatchBinaryName, scancelBinaryName} {
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

// JobInfo contains information about a single Slurm job.
type JobInfo struct {
	ID         int            `json:"id" slurm:"JobId"`
	UserID     int            `json:"user_id" slurm:"UserId"`
	Name       string         `json:"name" slurm:"JobName"`
	ExitCode   int            `json:"exit_code" slurm:"ExitCode"`
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
	NumNodes   int            `json:"num_nodes" slurm:"NumNodes"`
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

func (*Client) SJobInfo(jobID int64) (*JobInfo, error) {
	var cJobInfoMsg *C.job_info_msg_t
	err := C.slurm_load_job(&cJobInfoMsg, C.uint32_t(jobID), C.uint16_t(0))
	defer C.slurm_free_job_info_msg(cJobInfoMsg)
	if err != C.SLURM_SUCCESS {
		errNo := C.slurm_get_errno()
		errMsg := C.slurm_strerror(errNo)
		return nil, errors.New(C.GoString(errMsg))
	}
	if cJobInfoMsg.record_count == 0 {
		return nil, errors.New("slurm_load_job returned empty job array")
	}
	log.Printf("slurm_load_job returned %v records", cJobInfoMsg.record_count)
	log.Printf("slurm_load_job returned %+v", cJobInfoMsg.job_array)

	// data := unsafe.Pointer(cJobInfoMsg.job_array)
	// count := int(cJobInfoMsg.record_count)
	// carray := *(*[]C.job_info_t)(unsafe.Pointer(&reflect.SliceHeader{
	// 	Data: uintptr(data),
	// 	Len:  count,
	// 	Cap:  count,
	// }))
	//
	// res := get_res(slres)
	//
	// array := make([]*table, count)
	// for i := 0; i < count; i++ {
	// 	array[i] = get_res(&carray[i])
	// }
	//
	// (*res)["JobArray"] = array
	//
	// C.slurm_free_job_info_msg(slres)
	//
	// w.Header().Set("Content-Type", "application/json")
	// json.NewEncoder(w).Encode(&res)

	// slice := (*[1 << 28]C.slurm_job_info_t)(unsafe.Pointer(cJobInfoMsg.job_array))[:cJobInfoMsg.record_count:cJobInfoMsg.record_count]
	// cJobInfo := slice[0]
	// log.Printf("job info: %+v", cJobInfo)
	//
	// ji := &slurm.JobInfo{
	// 	ID:         int(cJobInfo.job_id),
	// 	UserID:     int(cJobInfo.user_id),
	// 	Name:       C.GoString(cJobInfo.name),
	// 	ExitCode:   int(cJobInfo.exit_code),
	// 	State:      "",
	// 	SubmitTime: nil,
	// 	StartTime:  nil,
	// 	RunTime:    nil,
	// 	TimeLimit:  nil,
	// 	WorkDir:    C.GoString(cJobInfo.work_dir),
	// 	StdOut:     C.GoString(cJobInfo.std_out),
	// 	StdErr:     C.GoString(cJobInfo.std_err),
	// 	Partition:  C.GoString(cJobInfo.partition),
	// 	NodeList:   C.GoString(cJobInfo.nodes),
	// 	BatchHost:  C.GoString(cJobInfo.batch_host),
	// 	NumNodes:   int(cJobInfo.num_nodes),
	// }

	var ji *JobInfo
	log.Printf("Job info: %v", ji)
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

		startedAt, err := ParseTime(splitted[0])
		if err != nil {
			return nil, err
		}

		finishedAt, err := ParseTime(splitted[1])
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

func ParseDuration(durationStr string) (*time.Duration, error) {
	sp := strings.Split(durationStr, ":")
	if len(sp) < 3 {
		// we can skip since data is invalid or not available for that field
		return nil, nil
	}

	if strings.Contains(sp[0], "-") {
		spl := strings.Split(sp[0], "-")
		days, err := strconv.ParseInt(spl[0], 10, 0)
		if err != nil {
			return nil, err
		}

		hours, err := strconv.ParseInt(spl[1], 10, 0)
		if err != nil {
			return nil, err
		}

		sp[0] = strconv.FormatInt(days*24+hours, 10)
	}

	d, err := time.ParseDuration(fmt.Sprintf("%sh%sm%ss", sp[0], sp[1], sp[2]))
	return &d, err
}

func ParseTime(timeStr string) (*time.Time, error) {
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
