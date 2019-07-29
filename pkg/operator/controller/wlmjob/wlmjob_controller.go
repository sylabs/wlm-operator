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
	"context"
	"os"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	wlmv1alpha1 "github.com/sylabs/wlm-operator/pkg/operator/apis/wlm/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Reconciler reconciles a WlmJob object.
type Reconciler struct {
	client client.Client
	scheme *runtime.Scheme

	jcUID int64
	jcGID int64
}

// NewReconciler returns a new WlmJob controller.
func NewReconciler(mgr manager.Manager) *Reconciler {
	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		jcUID:  int64(os.Getuid()),
		jcGID:  int64(os.Getgid()),
	}
	return r
}

// AddToManager adds WlmJob Reconciler to the given Manager.
// The Manager will set fields on the Reconciler and Start it when the Manager is Started.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	c, err := controller.New("wlmjob-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource WlmJob
	err = c.Watch(&source.Kind{Type: &wlmv1alpha1.WlmJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner WlmJob
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &wlmv1alpha1.WlmJob{},
	})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a WlmJob object and makes changes
// based on the state read and what is in the WlmJob.Spec.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	glog.Infof("Received reconcile request: %v", req)

	// Fetch the WlmJob instance
	wj := &wlmv1alpha1.WlmJob{}
	err := r.client.Get(context.Background(), req.NamespacedName, wj)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		glog.Errorf("Could not get wlm job: %v", err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Translate WlmJob to Pod
	sjPod, err := r.newPodForWJ(wj)
	if err != nil {
		glog.Errorf("Could not translate wlm job into pod: %v", err)
		return reconcile.Result{}, err
	}

	// Set WlmJob instance as the owner and controller
	err = controllerutil.SetControllerReference(wj, sjPod, r.scheme)
	if err != nil {
		glog.Errorf("Could not set controller reference for pod: %v", err)
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	wjCurrentPod := &corev1.Pod{}
	key := types.NamespacedName{Name: sjPod.Name, Namespace: sjPod.Namespace}
	err = r.client.Get(context.Background(), key, wjCurrentPod)
	if err != nil && errors.IsNotFound(err) {
		if wj.Status.Status != "" {
			glog.Info("Pod will not be created, it was already created once")
			return reconcile.Result{}, nil
		}

		glog.Infof("Creating new pod %q for wlm job %q", sjPod.Name, wj.Name)
		err = r.client.Create(context.Background(), sjPod)
		if err != nil {
			glog.Errorf("Could not create new pod: %v", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	glog.Infof("Updating wlm job %q", wj.Name)
	// Otherwise smth has changed, need to update things
	wj.Status.Status = string(wjCurrentPod.Status.Phase)
	err = r.client.Status().Update(context.Background(), wj)
	if err != nil {
		glog.Errorf("Could not update wlm job: %v", err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
