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
	"os"

	"github.com/golang/glog"
	wlmv1alpha1 "github.com/sylabs/wlm-operator/pkg/operator/apis/wlm/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

// Reconciler reconciles a SlurmJob object
type Reconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	jcUID int64
	jcGID int64
}

// NewReconciler returns a new SlurmJob controller.
func NewReconciler(mgr manager.Manager) *Reconciler {
	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		jcUID:  int64(os.Getuid()),
		jcGID:  int64(os.Getgid()),
	}
	return r
}

// AddToManager adds SlurmJob Reconciler to the given Manager.
// The Manager will set fields on the Reconciler and Start it when the Manager is Started.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	// Create a new controller
	c, err := controller.New("slurmjob-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SlurmJob
	err = c.Watch(&source.Kind{Type: &wlmv1alpha1.SlurmJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner SlurmJob
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &wlmv1alpha1.SlurmJob{},
	})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a SlurmJob object and makes changes
// based on the state read and what is in the SlurmJob.Spec.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	glog.Infof("Received reconcile request: %v", req)

	// Fetch the SlurmJob instance
	sj := &wlmv1alpha1.SlurmJob{}
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
	sjPod, err := r.newPodForSJ(sj)
	if err != nil {
		glog.Errorf("Could not translate slurm job into pod: %v", err)
		return reconcile.Result{}, err
	}

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
		if sj.Status.Status != "" {
			glog.Info("Pod will not be created, it was already created once")
			return reconcile.Result{}, nil
		}

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
