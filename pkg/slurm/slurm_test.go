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

func TestParseSacctResponse(t *testing.T) {
	tt := []struct {
		name        string
		in          string
		expect      []*JobInfo
		expectError string
	}{
		{
			name: "single line",
			in:   "2019-02-20T11:16:55|2019-02-20T11:16:55|2:0|COMPLETED|comm|35|test|",
			expect: []*JobInfo{
				{
					ID:         "35",
					Name:       "test",
					StartedAt:  time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC),
					FinishedAt: time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC),
					ExitCode:   2,
					State:      "COMPLETED",
					Comment:    "comm",
				},
			},
		},
		{
			name: "multi line",
			in: `2019-02-20T11:16:55|2019-02-20T11:16:55|0:0|COMPLETED|c|35|test|
2019-02-20T11:16:55|2019-02-20T11:16:55|0:0|COMPLETED|c|35.0|sleep|
2019-02-20T11:16:55|unknown|0:0|COMPLETED|some comment|35.1|echo 'lala'|`,
			expect: []*JobInfo{
				{
					ID:         "35",
					Name:       "test",
					StartedAt:  time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC),
					FinishedAt: time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC),
					ExitCode:   0,
					State:      "COMPLETED",
					Comment:    "c",
				},
				{
					ID:         "35.0",
					Name:       "sleep",
					StartedAt:  time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC),
					FinishedAt: time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC),
					ExitCode:   0,
					State:      "COMPLETED",
					Comment:    "c",
				},
				{
					ID:         "35.1",
					Name:       "echo 'lala'",
					StartedAt:  time.Date(2019, 2, 20, 11, 16, 55, 0, time.UTC),
					FinishedAt: time.Time{},
					ExitCode:   0,
					State:      "COMPLETED",
					Comment:    "some comment",
				},
			},
		},
		{
			name:        "invalid start time",
			in:          "20 Feb 20109 11:16:55|2019-02-20T11:16:55|2:0|COMPLETED|comm|35|test|",
			expect:      nil,
			expectError: "parsing time \"20 Feb 20109 11:16:55\" as \"2006-01-02T15:04:05\": cannot parse \"eb 20109 11:16:55\" as \"2006\"",
		},
		{
			name:        "invalid end time",
			in:          "2019-02-20T11:16:55|20 Feb 20109 11:16:55|2:0|COMPLETED|comm|35|test|",
			expect:      nil,
			expectError: "parsing time \"20 Feb 20109 11:16:55\" as \"2006-01-02T15:04:05\": cannot parse \"eb 20109 11:16:55\" as \"2006\"",
		},
		{
			name:        "invalid exit code",
			in:          "2019-02-20T11:16:55|2019-02-20T11:16:55|2:5:0|COMPLETED|comm|35|test|",
			expect:      nil,
			expectError: "exit code must contain 2 sections",
		},
		{
			name:        "string exit code",
			in:          "2019-02-20T11:16:55|2019-02-20T11:16:55|F:0|COMPLETED|comm|35|test|",
			expect:      nil,
			expectError: "strconv.Atoi: parsing \"F\": invalid syntax",
		},
		{
			name: "invalid format",
			in: `sacct: error: slurmdb_ave_tres_usage: couldn't make tres_list from '0=0,1=942080,6=210386944,7=0'
2019-04-09T06:32:06|2019-04-09T06:32:08|0:0|COMPLETED||6|sbatch|
`,
			expect:      nil,
			expectError: "output must contain 7 sections",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := ParseSacctResponse(tc.in)
			if tc.expectError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectError)
			}
			require.Equal(t, tc.expect, actual)
		})
	}
}
