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

const (
	testScontrolResponse = `JobId=53 JobName=sbatch
   UserId=vagrant(1000) GroupId=vagrant(1000) MCS_label=N/A
   Priority=4294901743 Nice=0 Account=(null) QOS=(null)
   JobState=RUNNING Reason=None Dependency=(null)
   Requeue=1 Restarts=0 BatchFlag=1 Reboot=0 ExitCode=0:0
   RunTime=00:00:30 TimeLimit=1-01:00:00 TimeMin=N/A
   SubmitTime=2019-04-16T11:49:19 EligibleTime=2019-04-16T11:49:19
   StartTime=2019-04-16T11:49:20 EndTime=2019-04-16T12:49:20 Deadline=N/A
   PreemptTime=None SuspendTime=None SecsPreSuspend=0
   LastSchedEval=2019-04-16T11:49:20
   Partition=debug AllocNode:Sid=vagrant:23733
   ReqNodeList=(null) ExcNodeList=(null)
   NodeList=vagrant
   BatchHost=vagrant
   NumNodes=1 NumCPUs=2 NumTasks=1 CPUs/Task=1 ReqB:S:C:T=0:0:*:*
   TRES=cpu=2,node=1,billing=2
   Socks/Node=* NtasksPerN:B:S:C=0:0:*:* CoreSpec=*
   MinCPUsNode=1 MinMemoryNode=0 MinTmpDiskNode=0
   Features=(null) DelayBoot=00:00:00
   Gres=(null) Reservation=(null)
   OverSubscribe=NO Contiguous=0 Licenses=(null) Network=(null)
   Command=(null)
   WorkDir=/home/vagrant
   StdErr=/home/vagrant/slurm-53.out
   StdIn=/dev/null
   StdOut=/home/vagrant/slurm-53.out
   Power=`

	testPendingScontrolRsponse = `JobId=52 JobName=sbatch
   UserId=vagrant(1000) GroupId=vagrant(1000) MCS_label=N/A
   Priority=4294901744 Nice=0 Account=(null) QOS=(null)
   JobState=PENDING Reason=None Dependency=(null)
   Requeue=1 Restarts=0 BatchFlag=1 Reboot=0 ExitCode=0:0
   RunTime=00:00:00 TimeLimit=UNLIMITED TimeMin=N/A
   SubmitTime=2019-04-16T11:49:19 EligibleTime=2019-04-16T11:48:02
   StartTime=Unknown EndTime=Unknown Deadline=N/A
   PreemptTime=None SuspendTime=None SecsPreSuspend=0
   LastSchedEval=2019-04-16T11:48:02
   Partition=debug AllocNode:Sid=vagrant:23733
   ReqNodeList=(null) ExcNodeList=(null)
   NodeList=(null)
   NumNodes=1 NumCPUs=1 NumTasks=1 CPUs/Task=1 ReqB:S:C:T=0:0:*:*
   TRES=cpu=1,node=1
   Socks/Node=* NtasksPerN:B:S:C=0:0:*:* CoreSpec=*
   MinCPUsNode=1 MinMemoryNode=0 MinTmpDiskNode=0
   Features=(null) DelayBoot=00:00:00
   Gres=(null) Reservation=(null)
   OverSubscribe=NO Contiguous=0 Licenses=(null) Network=(null)
   Command=(null)
   WorkDir=/home/vagrant
   StdErr=/home/vagrant/slurm-52.out
   StdIn=/dev/null
   StdOut=/home/vagrant/slurm-52.out
   Power=`

	testJobArrayScontrolResponse = `JobId=192 ArrayJobId=192 ArrayTaskId=5-8 JobName=sbatch
   UserId=vagrant(1000) GroupId=vagrant(1000) MCS_label=N/A
   Priority=4294901702 Nice=0 Account=(null) QOS=(null)
   JobState=PENDING Reason=Resources Dependency=(null)
   Requeue=1 Restarts=0 BatchFlag=1 Reboot=0 ExitCode=0:0
   RunTime=00:00:30 TimeLimit=1-01:00:00 TimeMin=N/A
   SubmitTime=2019-04-16T11:49:19 EligibleTime=2019-04-16T11:48:02
   StartTime=2019-04-16T11:49:20 EndTime=Unknown Deadline=N/A
   PreemptTime=None SuspendTime=None SecsPreSuspend=0
   LastSchedEval=2019-05-17T11:14:42
   Partition=debug AllocNode:Sid=vagrant:7471
   ReqNodeList=(null) ExcNodeList=(null)
   NodeList=(null)
   NumNodes=1-1 NumCPUs=1 NumTasks=1 CPUs/Task=1 ReqB:S:C:T=0:0:*:*
   TRES=cpu=1,node=1
   Socks/Node=* NtasksPerN:B:S:C=0:0:*:* CoreSpec=*
   MinCPUsNode=1 MinMemoryNode=0 MinTmpDiskNode=0
   Features=(null) DelayBoot=00:00:00
   Gres=(null) Reservation=(null)
   OverSubscribe=NO Contiguous=0 Licenses=(null) Network=(null)
   Command=(null)
   WorkDir=/home/vagrant
   StdErr=/home/vagrant/slurm-192_4294967294.out
   StdIn=/dev/null
   StdOut=/home/vagrant/slurm-192_4294967294.out
   Power=

JobId=196 ArrayJobId=192 ArrayTaskId=4 JobName=sbatch
   UserId=vagrant(1000) GroupId=vagrant(1000) MCS_label=N/A
   Priority=4294901702 Nice=0 Account=(null) QOS=(null)
   JobState=RUNNING Reason=None Dependency=(null)
   Requeue=1 Restarts=0 BatchFlag=1 Reboot=0 ExitCode=0:0
   RunTime=00:00:30 TimeLimit=1-01:00:00 TimeMin=N/A
   SubmitTime=2019-04-16T11:49:19 EligibleTime=2019-04-16T11:49:19
   StartTime=2019-04-16T11:49:20 EndTime=2019-04-16T12:49:20 Deadline=N/A
   PreemptTime=None SuspendTime=None SecsPreSuspend=0
   LastSchedEval=2019-05-17T11:13:59
   Partition=debug AllocNode:Sid=vagrant:7471
   ReqNodeList=(null) ExcNodeList=(null)
   NodeList=vagrant
   BatchHost=vagrant
   NumNodes=1 NumCPUs=2 NumTasks=1 CPUs/Task=1 ReqB:S:C:T=0:0:*:*
   TRES=cpu=2,node=1,billing=2
   Socks/Node=* NtasksPerN:B:S:C=0:0:*:* CoreSpec=*
   MinCPUsNode=1 MinMemoryNode=0 MinTmpDiskNode=0
   Features=(null) DelayBoot=00:00:00
   Gres=(null) Reservation=(null)
   OverSubscribe=NO Contiguous=0 Licenses=(null) Network=(null)
   Command=(null)
   WorkDir=/home/vagrant
   StdErr=/home/vagrant/slurm-192_4.out
   StdIn=/dev/null
   StdOut=/home/vagrant/slurm-192_4.out
   Power=`

	testScontrolShowPartition = `
	PartitionName=debug
   AllowGroups=ALL AllowAccounts=ALL AllowQos=ALL
   AllocNodes=ALL Default=YES QoS=N/A
   DefaultTime=NONE DisableRootJobs=NO ExclusiveUser=NO GraceTime=0 Hidden=NO
   MaxNodes=3 MaxTime=00:30:00 MinNodes=1 LLN=NO MaxCPUsPerNode=1
   Nodes=vagrant
   PriorityJobFactor=1 PriorityTier=1 RootOnly=NO ReqResv=NO OverSubscribe=NO
   OverTimeLimit=NONE PreemptMode=OFF
   State=UP TotalCPUs=2 TotalNodes=8 SelectTypeParameters=NONE
   DefMemPerNode=UNLIMITED MaxMemPerNode=512`

	testScontrolShowPartitionUnlimited = `
	PartitionName=debug
   AllowGroups=ALL AllowAccounts=ALL AllowQos=ALL
   AllocNodes=ALL Default=YES QoS=N/A
   DefaultTime=NONE DisableRootJobs=NO ExclusiveUser=NO GraceTime=0 Hidden=NO
   MaxNodes=UNLIMITED MaxTime=UNLIMITED MinNodes=1 LLN=NO MaxCPUsPerNode=UNLIMITED
   Nodes=vagrant
   PriorityJobFactor=1 PriorityTier=1 RootOnly=NO ReqResv=NO OverSubscribe=NO
   OverTimeLimit=NONE PreemptMode=OFF
   State=UP TotalCPUs=2 TotalNodes=4 SelectTypeParameters=NONE
   DefMemPerNode=UNLIMITED MaxMemPerNode=UNLIMITED
`
)

var (
	testSubmitTime  = time.Date(2019, 04, 16, 11, 49, 19, 0, time.UTC)
	testStartTime   = time.Date(2019, 04, 16, 11, 49, 20, 0, time.UTC)
	testSacctTime   = time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC)
	testRunTime     = 30 * time.Second
	testLimitTime   = 25 * time.Hour
	testZeroRunTime = time.Duration(0)
)

func TestJobInfoFromScontrolResponse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []*JobInfo
	}{
		{
			name: "t1",
			in:   testScontrolResponse,
			want: []*JobInfo{
				{
					ID:         "53",
					UserID:     "vagrant(1000)",
					Name:       "sbatch",
					ExitCode:   "0:0",
					State:      "RUNNING",
					SubmitTime: &testSubmitTime,
					StartTime:  &testStartTime,
					RunTime:    &testRunTime,
					TimeLimit:  &testLimitTime,
					WorkDir:    "/home/vagrant",
					StdOut:     "/home/vagrant/slurm-53.out",
					StdErr:     "/home/vagrant/slurm-53.out",
					Partition:  "debug",
					NodeList:   "vagrant",
					BatchHost:  "vagrant",
					NumNodes:   "1",
					ArrayJobID: "",
				},
			},
		},
		{
			name: "t2",
			in:   testPendingScontrolRsponse,
			want: []*JobInfo{
				{
					ID:         "52",
					UserID:     "vagrant(1000)",
					Name:       "sbatch",
					ExitCode:   "0:0",
					State:      "PENDING",
					SubmitTime: &testSubmitTime,
					StartTime:  nil,
					RunTime:    &testZeroRunTime,
					TimeLimit:  nil,
					WorkDir:    "/home/vagrant",
					StdOut:     "/home/vagrant/slurm-52.out",
					StdErr:     "/home/vagrant/slurm-52.out",
					Partition:  "debug",
					NodeList:   "(null)",
					BatchHost:  "",
					NumNodes:   "1",
					ArrayJobID: "",
				},
			},
		},
		{
			name: "t3",
			in:   testJobArrayScontrolResponse,
			want: []*JobInfo{
				{
					ID:         "192",
					UserID:     "vagrant(1000)",
					Name:       "sbatch",
					ExitCode:   "0:0",
					State:      "PENDING",
					SubmitTime: &testSubmitTime,
					StartTime:  &testStartTime,
					RunTime:    &testRunTime,
					TimeLimit:  &testLimitTime,
					WorkDir:    "/home/vagrant",
					StdOut:     "/home/vagrant/slurm-192_4294967294.out",
					StdErr:     "/home/vagrant/slurm-192_4294967294.out",
					Partition:  "debug",
					NodeList:   "(null)",
					BatchHost:  "",
					NumNodes:   "1-1",
					ArrayJobID: "192",
				},
				{
					ID:         "196",
					UserID:     "vagrant(1000)",
					Name:       "sbatch",
					ExitCode:   "0:0",
					State:      "RUNNING",
					SubmitTime: &testSubmitTime,
					StartTime:  &testStartTime,
					RunTime:    &testRunTime,
					TimeLimit:  &testLimitTime,
					WorkDir:    "/home/vagrant",
					StdOut:     "/home/vagrant/slurm-192_4.out",
					StdErr:     "/home/vagrant/slurm-192_4.out",
					Partition:  "debug",
					NodeList:   "vagrant",
					BatchHost:  "vagrant",
					NumNodes:   "1",
					ArrayJobID: "192",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JobInfoFromScontrolResponse(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
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
				CpuPerNode: 1,
				WallTime:   30 * time.Minute,
				Features:   nil,
			}},
		{
			name: "t2",
			in:   testScontrolShowPartitionUnlimited,
			want: &Resources{
				Nodes:      4,
				MemPerNode: -1,
				CpuPerNode: 2,
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
