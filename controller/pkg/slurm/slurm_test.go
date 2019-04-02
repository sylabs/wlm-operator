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

func TestSaact_parseSacctResponse(t *testing.T) {
	testOut := "2019-02-20T11:16:55|2019-02-20T11:16:55|2:0|COMPLETED|comm|35|test|"
	info, err := ParseSacctResponse(testOut)
	require.NoError(t, err)
	require.NotNil(t, info)

	ti, err := time.Parse(time.RFC3339, "2019-02-20T11:16:55Z")
	require.NoError(t, err)
	require.Len(t, info, 1)
	require.EqualValues(t, ti, info[0].StartedAt)
	require.EqualValues(t, ti, info[0].FinishedAt)
	require.EqualValues(t, 2, info[0].ExitCode)
	require.EqualValues(t, "COMPLETED", info[0].State)
	require.EqualValues(t, "comm", info[0].Comment)
	require.EqualValues(t, "35", info[0].ID)
	require.EqualValues(t, "test", info[0].Name)
}

func TestSaact_parseSacctMultilineResponse(t *testing.T) {
	testOut := `2019-02-20T11:16:55|2019-02-20T11:16:55|0:0|COMPLETED|c|35|test|
2019-02-20T11:16:55|2019-02-20T11:16:55|0:0|COMPLETED|c|35.0|sleep|`

	info, err := ParseSacctResponse(testOut)
	require.NoError(t, err)
	require.NotNil(t, info)

	ti, err := time.Parse(time.RFC3339, "2019-02-20T11:16:55Z")
	require.NoError(t, err)
	require.Len(t, info, 2)
	require.EqualValues(t, ti, info[0].StartedAt)
	require.EqualValues(t, ti, info[1].StartedAt)
	require.EqualValues(t, ti, info[0].FinishedAt)
	require.EqualValues(t, ti, info[1].FinishedAt)
	require.EqualValues(t, 0, info[0].ExitCode)
	require.EqualValues(t, 0, info[1].ExitCode)
	require.EqualValues(t, "COMPLETED", info[0].State)
	require.EqualValues(t, "COMPLETED", info[1].State)
	require.EqualValues(t, "c", info[0].Comment)
	require.EqualValues(t, "c", info[1].Comment)
	require.EqualValues(t, "35", info[0].ID)
	require.EqualValues(t, "35.0", info[1].ID)
	require.EqualValues(t, "test", info[0].Name)
	require.EqualValues(t, "sleep", info[1].Name)
}
