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

package slurmjob

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/sylabs/slurm-operator/pkg/slurm"
)

func TestExtractBatchResources(t *testing.T) {
	tt := []struct {
		name            string
		script          string
		expectResources *slurm.Resources
		expectError     error
	}{
		{
			name: "no resources",
			script: `
#!/bin/sh
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectResources: &slurm.Resources{},
		},
		{
			name: "wall time",
			script: `
#!/bin/sh
#SBATCH --time=00:05:00
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectResources: &slurm.Resources{
				WallTime: time.Minute * 5,
			},
		},
		{
			name: "nodes",
			script: `
#!/bin/sh
#SBATCH -t=00:05:00 --nodes 5
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectResources: &slurm.Resources{
				WallTime: time.Minute * 5,
				Nodes:    5,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			resources, err := extractBatchResources(tc.script)
			require.Equal(t, tc.expectError, err)
			require.Equal(t, tc.expectResources, resources)
		})
	}
}
