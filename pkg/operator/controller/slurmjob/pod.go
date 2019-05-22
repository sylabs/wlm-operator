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
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	slurmv1alpha1 "github.com/sylabs/slurm-operator/pkg/operator/apis/slurm/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newPodForSJ returns a job-companion pod for the slurm job.
func (r *Reconciler) newPodForSJ(sj *slurmv1alpha1.SlurmJob) (*corev1.Pod, error) {
	affinity, err := affinityForSj(sj)
	if err != nil {
		return nil, errors.Wrap(err, "could not form slurm job pod affinity")
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sj.Name + "-job",
			Namespace: sj.Namespace,
			Labels: map[string]string{
				"slurm-job": sj.Name,
			},
		},
		Spec: corev1.PodSpec{
			Affinity: affinity,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  &r.jcUID,
				RunAsGroup: &r.jcGID,
			},
			Containers: []corev1.Container{
				{
					Name:            "jt1",
					Image:           jobCompanionImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args:            companionArgsForSj(sj),
					Env: []corev1.EnvVar{
						{
							Name: "JOB_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.name",
								},
							},
						}},
					VolumeMounts: volumesMountForSj(sj),
				},
			},
			Volumes:       volumesForSj(sj),
			NodeSelector:  nodeSelectorForSj(sj),
			HostNetwork:   true,
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, nil
}

func affinityForSj(sj *slurmv1alpha1.SlurmJob) (*corev1.Affinity, error) {
	requiredResources, err := extractBatchResources(sj.Spec.Batch)
	if err != nil {
		return nil, errors.Wrap(err, "could not extract required resources")
	}
	var nodeMatch []corev1.NodeSelectorRequirement
	if requiredResources.Nodes != 0 {
		nodeMatch = append(nodeMatch, corev1.NodeSelectorRequirement{
			Key:      "slurm.sylabs.io/nodes",
			Operator: "Gt",
			Values:   []string{strconv.FormatInt(requiredResources.Nodes-1, 10)},
		})
	}
	if requiredResources.WallTime != 0 {
		nodeMatch = append(nodeMatch, corev1.NodeSelectorRequirement{
			Key:      "slurm.sylabs.io/time-limit",
			Operator: "Gt",
			Values:   []string{strconv.FormatInt(int64(requiredResources.WallTime/time.Second)-1, 10)},
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

func nodeSelectorForSj(sj *slurmv1alpha1.SlurmJob) map[string]string {
	// since we are running only slurm jobs, we need to be
	// sure that pod will be allocated only on nodes with slurm support
	nodeSelector := map[string]string{
		"slurm.sylabs.io/workload-manager": "slurm",
	}
	for k, v := range sj.Spec.NodeSelector {
		nodeSelector[k] = v
	}
	return nodeSelector
}

func companionArgsForSj(sj *slurmv1alpha1.SlurmJob) []string {
	args := []string{
		fmt.Sprintf("--batch=%s", sj.Spec.Batch),
	}
	if sj.Spec.Results != nil {
		args = append(args, fmt.Sprintf("--cr-mount=%s", "/collect"))
		if sj.Spec.Results.From != "" {
			args = append(args, fmt.Sprintf("--file-to-collect=%s", sj.Spec.Results.From))
		}
	}
	return args
}

func volumesForSj(sj *slurmv1alpha1.SlurmJob) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "red-box-sock",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/syslurm/red-box.sock",
					Type: &[]corev1.HostPathType{corev1.HostPathSocket}[0],
				},
			},
		},
	}

	if sj.Spec.Results != nil {
		volumes = append(volumes, sj.Spec.Results.Mount)
	}
	return volumes
}

func volumesMountForSj(sj *slurmv1alpha1.SlurmJob) []corev1.VolumeMount {
	// default SLURM config which have to exist on every k8s node. The config is managed and created by RD
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "red-box-sock",
			MountPath: "/red-box.sock",
		},
	}

	if sj.Spec.Results != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      sj.Spec.Results.Mount.Name,
			MountPath: "/collect",
		})
	}
	return volumeMounts
}
