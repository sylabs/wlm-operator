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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var testSacctTime = time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC)

func TestParseDuration(t *testing.T) {
	tt := []struct {
		in             string
		expectDuration *time.Duration
		expectError    bool
	}{
		{
			in:          "UNLIMITED",
			expectError: true,
		},
		{
			in:          "",
			expectError: true,
		},
		{
			in:          "6:6:6:6",
			expectError: true,
		},
		{
			in:             "6",
			expectDuration: &[]time.Duration{time.Minute * 6}[0],
		},
		{
			in:          "foo",
			expectError: true,
		},
		{
			in:             "6:06",
			expectDuration: &[]time.Duration{time.Minute*6 + time.Second*6}[0],
		},
		{
			in:          "foo:06",
			expectError: true,
		},
		{
			in:          "6:foo",
			expectError: true,
		},
		{
			in:             "6:06:06",
			expectDuration: &[]time.Duration{time.Hour*6 + time.Minute*6 + time.Second*6}[0],
		},
		{
			in:          "foo:6:06",
			expectError: true,
		},
		{
			in:          "6:foo:06",
			expectError: true,
		},

		{
			in:          "6:06:foo",
			expectError: true,
		},
		{
			in:             "3-5",
			expectDuration: &[]time.Duration{time.Hour*24*3 + time.Hour*5}[0],
		},
		{
			in:          "foo-5",
			expectError: true,
		},
		{
			in:          "3-foo",
			expectError: true,
		},
		{
			in:             "3-5:07",
			expectDuration: &[]time.Duration{time.Hour*24*3 + time.Hour*5 + time.Minute*7}[0],
		},
		{
			in:          "3-5:foo",
			expectError: true,
		},
		{
			in:             "3-5:07:08",
			expectDuration: &[]time.Duration{time.Hour*24*3 + time.Hour*5 + time.Minute*7 + time.Second*8}[0],
		},
		{
			in:          "3-5:07:bar",
			expectError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			actual, err := ParseDuration(tc.in)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectDuration, actual)
		})
	}
}

func TestParseSacctResponse(t *testing.T) {
	tt := []struct {
		name        string
		in          string
		expect      []*JobStepInfo
		expectError string
	}{
		{
			name: "single line",
			in:   "2019-02-20T11:16:55|2019-02-20T11:16:55|2:0|COMPLETED|35|test|",
			expect: []*JobStepInfo{
				{
					ID:         "35",
					Name:       "test",
					StartedAt:  &testSacctTime,
					FinishedAt: &testSacctTime,
					ExitCode:   2,
					State:      "COMPLETED",
				},
			},
		},
		{
			name: "multi line",
			in: `2019-02-20T11:16:55|2019-02-20T11:16:55|0:0|COMPLETED|35|test|
2019-02-20T11:16:55|2019-02-20T11:16:55|0:0|COMPLETED|35.0|sleep|
2019-02-20T11:16:55|unknown|0:0|COMPLETED|35.1|echo 'lala'|`,
			expect: []*JobStepInfo{
				{
					ID:         "35",
					Name:       "test",
					StartedAt:  &testSacctTime,
					FinishedAt: &testSacctTime,
					ExitCode:   0,
					State:      "COMPLETED",
				},
				{
					ID:         "35.0",
					Name:       "sleep",
					StartedAt:  &testSacctTime,
					FinishedAt: &testSacctTime,
					ExitCode:   0,
					State:      "COMPLETED",
				},
				{
					ID:         "35.1",
					Name:       "echo 'lala'",
					StartedAt:  &testSacctTime,
					FinishedAt: nil,
					ExitCode:   0,
					State:      "COMPLETED",
				},
			},
		},
		{
			name:        "invalid start time",
			in:          "20 Feb 20109 11:16:55|2019-02-20T11:16:55|2:0|COMPLETED|35|test|",
			expect:      nil,
			expectError: "parsing time \"20 Feb 20109 11:16:55\" as \"2006-01-02T15:04:05\": cannot parse \"eb 20109 11:16:55\" as \"2006\"",
		},
		{
			name:        "invalid end time",
			in:          "2019-02-20T11:16:55|20 Feb 20109 11:16:55|2:0|COMPLETED|35|test|",
			expect:      nil,
			expectError: "parsing time \"20 Feb 20109 11:16:55\" as \"2006-01-02T15:04:05\": cannot parse \"eb 20109 11:16:55\" as \"2006\"",
		},
		{
			name:        "invalid exit code",
			in:          "2019-02-20T11:16:55|2019-02-20T11:16:55|2:5:0|COMPLETED|35|test|",
			expect:      nil,
			expectError: "exit code must contain 2 sections",
		},
		{
			name:        "string exit code",
			in:          "2019-02-20T11:16:55|2019-02-20T11:16:55|F:0|COMPLETED|35|test|",
			expect:      nil,
			expectError: "strconv.Atoi: parsing \"F\": invalid syntax",
		},
		{
			name: "invalid format",
			in: `sacct: error: slurmdb_ave_tres_usage: couldn't make tres_list from '0=0,1=942080,6=210386944,7=0'
2019-04-09T06:32:06|2019-04-09T06:32:08|0:0|COMPLETED|6|sbatch|
`,
			expect:      nil,
			expectError: "output must contain 6 sections",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseSacctResponse(tc.in)
			if tc.expectError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectError)
			}
			require.Equal(t, tc.expect, actual)
		})
	}
}

func Test_parseResources(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want *Resources
	}{
		{
			name: "t1",
			in:   testScontrolShowPartition,
			want: &Resources{
				Nodes:      3,
				MemPerNode: 512,
				CPUPerNode: 1,
				WallTime:   30 * time.Minute,
				Features:   nil,
			}},
		{
			name: "t2",
			in:   testScontrolShowPartitionUnlimited,
			want: &Resources{
				Nodes:      4,
				MemPerNode: -1,
				CPUPerNode: 2,
				WallTime:   -1,
				Features:   nil,
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResources(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

const testScontrolShowAllPartitions = `
PartitionName=debug
   AllowGroups=ALL AllowAccounts=ALL AllowQos=ALL
   AllocNodes=ALL Default=NO QoS=N/A
   DefaultTime=NONE DisableRootJobs=NO ExclusiveUser=NO GraceTime=0 Hidden=NO
   MaxNodes=1 MaxTime=00:30:00 MinNodes=1 LLN=NO MaxCPUsPerNode=2
   Nodes=node-1
   PriorityJobFactor=1 PriorityTier=1 RootOnly=NO ReqResv=NO OverSubscribe=NO
   OverTimeLimit=NONE PreemptMode=OFF
   State=UP TotalCPUs=2 TotalNodes=1 SelectTypeParameters=NONE
   DefMemPerNode=UNLIMITED MaxMemPerNode=512

PartitionName=debug2
   AllowGroups=ALL AllowAccounts=ALL AllowQos=ALL
   AllocNodes=ALL Default=NO QoS=N/A
   DefaultTime=NONE DisableRootJobs=NO ExclusiveUser=NO GraceTime=0 Hidden=NO
   MaxNodes=1 MaxTime=00:30:00 MinNodes=1 LLN=NO MaxCPUsPerNode=2
   Nodes=node-1
   PriorityJobFactor=1 PriorityTier=1 RootOnly=NO ReqResv=NO OverSubscribe=NO
   OverTimeLimit=NONE PreemptMode=OFF
   State=UP TotalCPUs=2 TotalNodes=1 SelectTypeParameters=NONE
   DefMemPerNode=UNLIMITED MaxMemPerNode=512

PartitionName=debug3
   AllowGroups=ALL AllowAccounts=ALL AllowQos=ALL
   AllocNodes=ALL Default=YES QoS=N/A
   DefaultTime=NONE DisableRootJobs=NO ExclusiveUser=NO GraceTime=0 Hidden=NO
   MaxNodes=1 MaxTime=00:30:00 MinNodes=1 LLN=NO MaxCPUsPerNode=2
   Nodes=node-1
   PriorityJobFactor=1 PriorityTier=1 RootOnly=NO ReqResv=NO OverSubscribe=NO
   OverTimeLimit=NONE PreemptMode=OFF
   State=UP TotalCPUs=2 TotalNodes=1 SelectTypeParameters=NONE
   DefMemPerNode=UNLIMITED MaxMemPerNode=512

`

func Test_parsePartitionsNames(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			"t1",
			testScontrolShowAllPartitions,
			[]string{"debug", "debug2", "debug3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePartitionsNames(tt.in)
			require.EqualValues(t, tt.want, got)
		})
	}
}
