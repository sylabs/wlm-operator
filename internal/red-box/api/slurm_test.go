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

package api

import (
	"testing"
	"time"

	"github.com/sylabs/slurm-operator/pkg/slurm"

	"github.com/stretchr/testify/require"
)

func Test_mapSInfoToProtoInfo(t *testing.T) {
	testInfo := slurm.JobInfo{
		ID:         "1",
		UserID:     "vagrant",
		Name:       "test.job",
		ExitCode:   "0:1",
		State:      "COMPLETED",
		SubmitTime: &[]time.Time{time.Now()}[0],
		StartTime:  &[]time.Time{time.Now().Add(1 * time.Second)}[0],
		RunTime:    &[]time.Duration{time.Second}[0],
		TimeLimit:  &[]time.Duration{time.Hour}[0],
		WorkDir:    "/home",
		StdOut:     "1",
		StdErr:     "2",
		Partition:  "debug",
		NodeList:   "node1",
		BatchHost:  "host1",
		NumNodes:   "2",
		ArrayJobID: "111",
	}
	pinfs, err := mapSInfoToProtoInfo([]*slurm.JobInfo{&testInfo})
	require.NoError(t, err)
	require.Len(t, pinfs, 1)
	pi := pinfs[0]

	require.EqualValues(t, testInfo.ID, pi.Id)
	require.EqualValues(t, testInfo.UserID, pi.UserId)
	require.EqualValues(t, testInfo.Name, pi.Name)
	require.EqualValues(t, testInfo.ExitCode, pi.ExitCode)
	require.EqualValues(t, testInfo.State, pi.Status.String())
	require.EqualValues(t, testInfo.SubmitTime.Nanosecond(), pi.SubmitTime.Nanos)
	require.EqualValues(t, testInfo.SubmitTime.Unix(), pi.SubmitTime.Seconds)
	require.EqualValues(t, testInfo.StartTime.Nanosecond(), pi.StartTime.Nanos)
	require.EqualValues(t, testInfo.StartTime.Unix(), pi.StartTime.Seconds)
	require.EqualValues(t, testInfo.RunTime.Seconds(), pi.RunTime.Seconds)
	require.EqualValues(t, testInfo.TimeLimit.Seconds(), pi.TimeLimit.Seconds)
	require.EqualValues(t, testInfo.WorkDir, pi.WorkingDir)
	require.EqualValues(t, testInfo.StdOut, pi.StdOut)
	require.EqualValues(t, testInfo.StdErr, pi.StdErr)
	require.EqualValues(t, testInfo.Partition, pi.Partition)
	require.EqualValues(t, testInfo.NodeList, pi.NodeList)
	require.EqualValues(t, testInfo.BatchHost, pi.BatchHost)
	require.EqualValues(t, testInfo.NumNodes, pi.NumNodes)
	require.EqualValues(t, testInfo.ArrayJobID, pi.ArrayId)
}

func Test_mapSStepsToProtoSteps(t *testing.T) {
	var steps = []*slurm.JobStepInfo{
		{
			ID:         "1",
			Name:       "job1",
			StartedAt:  &[]time.Time{time.Now()}[0],
			FinishedAt: &[]time.Time{time.Now()}[0],
			ExitCode:   1,
			State:      "FAILED",
		},
		{
			ID:         "2",
			Name:       "job2",
			StartedAt:  &[]time.Time{time.Now()}[0],
			FinishedAt: &[]time.Time{time.Now()}[0],
			ExitCode:   2,
			State:      "COMPLETED",
		},
	}

	pSteps, err := toProtoSteps(steps)
	require.NoError(t, err)
	require.Len(t, pSteps, 2)
	for i := range pSteps {
		require.EqualValues(t, steps[i].ID, pSteps[i].Id)
		require.EqualValues(t, steps[i].Name, pSteps[i].Name)
		require.EqualValues(t, steps[i].StartedAt.Unix(), pSteps[i].StartTime.Seconds)
		require.EqualValues(t, steps[i].FinishedAt.Unix(), pSteps[i].EndTime.Seconds)
		require.EqualValues(t, steps[i].ExitCode, pSteps[i].ExitCode)
		require.EqualValues(t, steps[i].State, pSteps[i].Status.String())
	}
}
