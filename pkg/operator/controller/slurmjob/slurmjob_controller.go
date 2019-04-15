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
	"context"
	"fmt"

	"github.com/golang/glog"
	slurmv1alpha1 "github.com/sylabs/slurm-operator/pkg/operator/apis/slurm/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	jobCompanionImage = "sylabsio/slurm:job-companion"
	slurmCfgPath      = "/syslurm/slurm-cfg.yaml"
)

// Add creates a new SlurmJob Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSlurmJob{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("slurmjob-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SlurmJob
	err = c.Watch(&source.Kind{Type: &slurmv1alpha1.SlurmJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner SlurmJob
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &slurmv1alpha1.SlurmJob{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileSlurmJob{}

// ReconcileSlurmJob reconciles a SlurmJob object
type ReconcileSlurmJob struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a SlurmJob object and makes changes
// based on the state read and what is in the SlurmJob.Spec.
func (r *ReconcileSlurmJob) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	glog.Infof("Received reconcile request: %v", req)

	// Fetch the SlurmJob instance
	sj := &slurmv1alpha1.SlurmJob{}
	err := r.client.Get(context.Background(), req.NamespacedName, sj)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		glog.Errorf("Could not get slurm job: %v", err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Translate SlurmJob to Pod
	sjPod := newPodForSJ(sj)

	// Set SlurmJob instance as the owner and controller
	err = controllerutil.SetControllerReference(sj, sjPod, r.scheme)
	if err != nil {
		glog.Errorf("Could not set controller reference for pod: %v", err)
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	sjCurrentPod := &corev1.Pod{}
	key := types.NamespacedName{Name: sjPod.Name, Namespace: sjPod.Namespace}
	err = r.client.Get(context.Background(), key, sjCurrentPod)
	if err != nil && errors.IsNotFound(err) {
		glog.Infof("Creating new pod %q for slurm job %q", sjPod.Name, sj.Name)
		err = r.client.Create(context.Background(), sjPod)
		if err != nil {
			glog.Errorf("Could not create new pod: %v", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	glog.Infof("Updating slurm job %q", sj.Name)
	// Otherwise smth has changed, need to update things
	sj.Status.Status = string(sjCurrentPod.Status.Phase)
	err = r.client.Status().Update(context.Background(), sj)
	if err != nil {
		glog.Errorf("Could not update slurm job: %v", err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// newPodForSJ returns a job-companion pod for the slurm job.
func newPodForSJ(sj *slurmv1alpha1.SlurmJob) *corev1.Pod {
	labels := map[string]string{
		"app": sj.Name,
	}

	// since we are running only slurm jobs, we need to be
	// sure that pod will be allocated only on nodes with slurm support
	selectorLabels := map[string]string{
		"slurm.sylabs.io/workload-manager": "slurm",
		"slurm.sylabs.io/integration-type": "local",
	}
	for k, v := range sj.Spec.NodeSelector {
		selectorLabels[k] = v
	}

	if sj.Spec.SSH != nil {
		selectorLabels["slurm.sylabs.io/integration-type"] = "ssh"
	}

	var resourceRequest corev1.ResourceList
	for k, v := range sj.Spec.Resources {
		if resourceRequest == nil {
			resourceRequest = make(map[corev1.ResourceName]resource.Quantity)
		}

		q := resource.NewQuantity(v, resource.DecimalSI)
		resourceRequest[corev1.ResourceName(k)] = *q
	}

	var ssh bool
	if sj.Spec.SSH != nil {
		ssh = true
	}
	args := []string{
		fmt.Sprintf("--batch=%s", sj.Spec.Batch),
		fmt.Sprintf("--config=%s", slurmCfgPath),
		fmt.Sprintf("--ssh=%t", ssh),
	}

	if sj.Spec.Results != nil {
		args = append(args, fmt.Sprintf("--cr-mount=%s", "/collect"))
		if sj.Spec.Results.From != "" {
			args = append(args, fmt.Sprintf("--file-to-collect=%s", sj.Spec.Results.From))
		}
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sj.Name + "-job",
			Namespace: sj.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			SecurityContext: sj.Spec.PodSecurityContext,
			Containers: []corev1.Container{
				{
					Name:            "jt1",
					Image:           jobCompanionImage,
					ImagePullPolicy: corev1.PullAlways,
					Args:            args,
					Resources: corev1.ResourceRequirements{
						Requests: resourceRequest,
						Limits:   resourceRequest,
					},
					Env:          getEnvs(sj),
					VolumeMounts: getVolumesMount(sj),
				},
			},
			Volumes:       getVolumes(sj),
			NodeSelector:  selectorLabels,
			HostNetwork:   true,
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func getVolumes(cr *slurmv1alpha1.SlurmJob) []corev1.Volume {
	var volumes []corev1.Volume

	hostPathType := corev1.HostPathDirectoryOrCreate // var since we need to have a ref on it
	volumes = append(volumes, corev1.Volume{
		Name: "slurm-cfg",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/lib/syslurm",
				Type: &hostPathType,
			}}})

	if cr.Spec.Results != nil {
		volumes = append(volumes, cr.Spec.Results.Mount)
	}

	return volumes
}

func getVolumesMount(cr *slurmv1alpha1.SlurmJob) []corev1.VolumeMount {
	var vms []corev1.VolumeMount
	// default SLRUM config which have to exist on every k8s node. The config is managed and created by RD
	vms = append(vms, corev1.VolumeMount{
		Name:      "slurm-cfg",
		ReadOnly:  true,
		MountPath: "/syslurm",
	})

	if cr.Spec.Results != nil {
		vms = append(vms, corev1.VolumeMount{
			Name:      cr.Spec.Results.Mount.Name,
			MountPath: "/collect",
		})
	}

	return vms
}

func getEnvs(cr *slurmv1alpha1.SlurmJob) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name: "JOB_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}

	if cr.Spec.SSH == nil {
		return envs
	}

	envs = append(envs, corev1.EnvVar{
		Name:  "SSH_USER",
		Value: cr.Spec.SSH.User,
	})

	if cr.Spec.SSH.Key != nil {
		envs = append(envs, corev1.EnvVar{
			Name:      "SSH_KEY",
			ValueFrom: cr.Spec.SSH.Key,
		})
	}

	if cr.Spec.SSH.Password != nil {
		envs = append(envs, corev1.EnvVar{
			Name:      "SSH_PASSWORD",
			ValueFrom: cr.Spec.SSH.Password,
		})
	}

	return envs
}
