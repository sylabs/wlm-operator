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
		expectError     bool
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
			name: "invalid wall time",
			script: `
#!/bin/sh
#SBATCH --time=invalid
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectError: true,
		},
		{
			name: "nodes",
			script: `
#!/bin/sh
#SBATCH -t=1-07 --nodes 5
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectResources: &slurm.Resources{
				WallTime: time.Hour * 31,
				Nodes:    5,
			},
		},
		{
			name: "nodes short",
			script: `
#!/bin/sh
#SBATCH --time 00:05:00

#SBATCH -N=5

srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectResources: &slurm.Resources{
				WallTime: time.Minute * 5,
				Nodes:    5,
			},
		},
		{
			name: "nodes min-max",
			script: `
#!/bin/sh
#SBATCH --time 00:05:00
#SBATCH --nodes=5-7
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectResources: &slurm.Resources{
				WallTime: time.Minute * 5,
				Nodes:    5,
			},
		},
		{
			name: "invalid nodes",
			script: `
#!/bin/sh
#SBATCH --time 00:05:00   -N=foo
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectError: true,
		},
		{
			name: "memory",
			script: `
#!/bin/sh
#SBATCH --mem 24000
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectResources: &slurm.Resources{
				MemPerNode: 24000,
			},
		},
		{
			name: "invalid memory",
			script: `
#!/bin/sh
#SBATCH --mem=foo
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
			expectError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			resources, err := extractBatchResources(tc.script)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectResources, resources)
		})
	}
}
