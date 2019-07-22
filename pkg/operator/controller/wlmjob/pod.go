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

package wlmjob

import (
	"time"

	"github.com/pkg/errors"
	wlmv1alpha1 "github.com/sylabs/wlm-operator/pkg/operator/apis/wlm/v1alpha1"
	"github.com/sylabs/wlm-operator/pkg/operator/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newPodForWJ returns a job-companion pod for the wlm job.
func (r *Reconciler) newPodForWJ(wj *wlmv1alpha1.WlmJob) (*corev1.Pod, error) {
	res := controller.Resources{
		Nodes:      wj.Spec.Resources.Nodes,
		MemPerNode: wj.Spec.Resources.MemPerNode,
		CPUPerNode: wj.Spec.Resources.CPUPerNode,
		WallTime:   time.Duration(wj.Spec.Resources.WallTime) * time.Second,
	}
	affinity, err := controller.AffinityForResources(res)
	if err != nil && err != controller.ErrAffinityIsNotRequired {
		return nil, errors.Wrap(err, "could not get job affinity")
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wj.Name + "-wlm-job",
			Namespace: wj.Namespace,
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  &r.jcUID,
				RunAsGroup: &r.jcGID,
			},
			Containers: []corev1.Container{
				{
					Name:  "jt2",
					Image: "no-image",
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Affinity:      affinity,
			Tolerations:   controller.DefaultTolerations,
			NodeSelector:  nodeSelectorForWj(wj),
		},
	}, nil
}

func nodeSelectorForWj(wj *wlmv1alpha1.WlmJob) map[string]string {
	nodeSelector := make(map[string]string)
	for k, v := range controller.DefaultNodeSelectors {
		nodeSelector[k] = v
	}

	for k, v := range wj.Spec.NodeSelector {
		nodeSelector[k] = v
	}
	return nodeSelector
}
