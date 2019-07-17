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
	"github.com/sylabs/wlm-operator/pkg/operator/controller"

	"github.com/pkg/errors"
	wlmv1alpha1 "github.com/sylabs/wlm-operator/pkg/operator/apis/wlm/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var errAffinityIsNotRequired = errors.New("affinity selectors is not required")

// newPodForSJ returns a job-companion pod for the slurm job.
func (r *Reconciler) newPodForSJ(sj *wlmv1alpha1.SlurmJob) (*corev1.Pod, error) {
	affinity, err := affinityForSj(sj)
	if err != nil && err != errAffinityIsNotRequired {
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
			Tolerations: controller.DefaultTolerations,
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

	return controller.AffinityForResources(*requiredResources)
}

func nodeSelectorForSj(sj *wlmv1alpha1.SlurmJob) map[string]string {
	nodeSelector := make(map[string]string)

	for k, v := range controller.DefaultNodeSelectors {
		nodeSelector[k] = v
	}

	for k, v := range sj.Spec.NodeSelector {
		nodeSelector[k] = v
	}
	return nodeSelector
}
