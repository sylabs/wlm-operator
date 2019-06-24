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

	"github.com/stretchr/testify/require"
	"github.com/sylabs/wlm-operator/pkg/operator/apis/wlm/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestAffinityForSj(t *testing.T) {
	tt := []struct {
		name           string
		sj             *v1alpha1.SlurmJob
		expectAffinity *corev1.Affinity
		expectError    bool
	}{
		{
			name: "invalid batch script",
			sj: &v1alpha1.SlurmJob{
				Spec: v1alpha1.SlurmJobSpec{
					Batch: `
#!/bin/sh
#SBATCH --time=foo
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
				},
			},
			expectError: true,
		},
		{
			name: "wall time affinity",
			sj: &v1alpha1.SlurmJob{
				Spec: v1alpha1.SlurmJobSpec{
					Batch: `
#!/bin/sh
#SBATCH --time=00:05:00
srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
				},
			},
			expectAffinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "wlm.sylabs.io/wall-time",
										Operator: "Gt",
										Values:   []string{"299"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "nodes affinity",
			sj: &v1alpha1.SlurmJob{
				Spec: v1alpha1.SlurmJobSpec{
					Batch: `
#!/bin/sh
#SBATCH -N 7

srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
				},
			},
			expectAffinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "wlm.sylabs.io/nodes",
										Operator: "Gt",
										Values:   []string{"6"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "memory affinity",
			sj: &v1alpha1.SlurmJob{
				Spec: v1alpha1.SlurmJobSpec{
					Batch: `
#!/bin/sh
#SBATCH --mem=120060

srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
				},
			},
			expectAffinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "wlm.sylabs.io/mem-per-node",
										Operator: "Gt",
										Values:   []string{"120059"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "cpus affinity",
			sj: &v1alpha1.SlurmJob{
				Spec: v1alpha1.SlurmJobSpec{
					Batch: `
#!/bin/sh
#SBATCH --cpus-per-task 8

srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
				},
			},
			expectAffinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "wlm.sylabs.io/cpu-per-node",
										Operator: "Gt",
										Values:   []string{"7"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "full affinity",
			sj: &v1alpha1.SlurmJob{
				Spec: v1alpha1.SlurmJobSpec{
					Batch: `
#!/bin/sh
#SBATCH -N 7  --mem=120060
#SBATCH --time=1-00:05:00

#SBATCH --cpus-per-task=8
#SBATCH --ntasks-per-node=2

srun singularity pull -U library://sylabsed/examples/lolcow
srun singularity run lolcow_latest.sif
srun rm lolcow_latest.sif
`,
				},
			},
			expectAffinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "wlm.sylabs.io/nodes",
										Operator: "Gt",
										Values:   []string{"6"},
									},
									{
										Key:      "wlm.sylabs.io/wall-time",
										Operator: "Gt",
										Values:   []string{"86699"},
									},
									{
										Key:      "wlm.sylabs.io/mem-per-node",
										Operator: "Gt",
										Values:   []string{"120059"},
									},
									{
										Key:      "wlm.sylabs.io/cpu-per-node",
										Operator: "Gt",
										Values:   []string{"15"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := affinityForSj(tc.sj)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectAffinity, actual)
		})
	}
}
