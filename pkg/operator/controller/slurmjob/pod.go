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
	"strconv"
	"time"

	"github.com/pkg/errors"
	wlmv1alpha1 "github.com/sylabs/wlm-operator/pkg/operator/apis/wlm/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newPodForSJ returns a job-companion pod for the slurm job.
func (r *Reconciler) newPodForSJ(sj *wlmv1alpha1.SlurmJob) (*corev1.Pod, error) {
	affinity, err := affinityForSj(sj)
	if err != nil {
		return nil, errors.Wrap(err, "could not form slurm job pod affinity")
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sj.Name + "-job",
			Namespace: sj.Namespace,
		},
		Spec: corev1.PodSpec{
			Affinity: affinity,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  &r.jcUID,
				RunAsGroup: &r.jcGID,
			},
			Tolerations: tolerationsForSj(sj),
			Containers: []corev1.Container{
				{
					Name:  "jt1",
					Image: "no-image",
				},
			},
			NodeSelector:  nodeSelectorForSj(sj),
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, nil
}

func affinityForSj(sj *wlmv1alpha1.SlurmJob) (*corev1.Affinity, error) {
	requiredResources, err := extractBatchResources(sj.Spec.Batch)
	if err != nil {
		return nil, errors.Wrap(err, "could not extract required resources")
	}
	var nodeMatch []corev1.NodeSelectorRequirement
	if requiredResources.Nodes != 0 {
		nodeMatch = append(nodeMatch, corev1.NodeSelectorRequirement{
			Key:      "wlm.sylabs.io/nodes",
			Operator: "Gt",
			Values:   []string{strconv.FormatInt(requiredResources.Nodes-1, 10)},
		})
	}
	if requiredResources.WallTime != 0 {
		nodeMatch = append(nodeMatch, corev1.NodeSelectorRequirement{
			Key:      "wlm.sylabs.io/wall-time",
			Operator: "Gt",
			Values:   []string{strconv.FormatInt(int64(requiredResources.WallTime/time.Second)-1, 10)},
		})
	}
	if requiredResources.MemPerNode != 0 {
		nodeMatch = append(nodeMatch, corev1.NodeSelectorRequirement{
			Key:      "wlm.sylabs.io/mem-per-node",
			Operator: "Gt",
			Values:   []string{strconv.FormatInt(requiredResources.MemPerNode-1, 10)},
		})
	}
	if requiredResources.CPUPerNode != 0 {
		nodeMatch = append(nodeMatch, corev1.NodeSelectorRequirement{
			Key:      "wlm.sylabs.io/cpu-per-node",
			Operator: "Gt",
			Values:   []string{strconv.FormatInt(requiredResources.CPUPerNode-1, 10)},
		})
	}
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: nodeMatch}},
			},
		},
	}, nil
}

func nodeSelectorForSj(sj *wlmv1alpha1.SlurmJob) map[string]string {
	// since we are running only slurm jobs, we need to be
	// sure that pod will be allocated only on nodes with slurm support
	nodeSelector := map[string]string{
		"type": "virtual-kubelet",
	}
	for k, v := range sj.Spec.NodeSelector {
		nodeSelector[k] = v
	}
	return nodeSelector
}

func tolerationsForSj(_ *wlmv1alpha1.SlurmJob) []corev1.Toleration {
	return []corev1.Toleration{
		{
			Key:      "virtual-kubelet.io/provider",
			Operator: corev1.TolerationOpEqual,
			Value:    "slurm",
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
}
