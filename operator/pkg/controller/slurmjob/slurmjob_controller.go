package slurmjob

import (
	"context"
	"fmt"

	slurmv1alpha1 "github.com/sylabs/slurm-operator/operator/pkg/apis/slurm/v1alpha1"
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
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	jobCompanionVersion = "0.0.38"

	imageName = "cali4888/jt1"

	slurmCfgPath = "/syslurm/slurm-cfg.yaml"
)

var log = logf.Log.WithName("controller_slurmjob")

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

// Reconcile reads that state of the cluster for a SlurmJob object and makes changes based on the state read
// and what is in the SlurmJob.Spec
// a Pod as an example
func (r *ReconcileSlurmJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SlurmJob")

	// Fetch the SlurmJob instance
	instance := &slurmv1alpha1.SlurmJob{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Define a new Pod object
	pod := newPodForCR(instance)

	// Set SlurmJob instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	}

	// Pod already exists - don't requeue
	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil
}

// newPodForCR returns a slurm-job-companion pod with the same name/namespace as the cr
func newPodForCR(cr *slurmv1alpha1.SlurmJob) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}

	// since we are running only slurm jobs, we need to be
	// sure that pod will be allocated only on nodes with slurm support
	selectorLabels := map[string]string{"slurm.sylabs.io/workload-manager": "slurm"}
	for k, v := range cr.Spec.NodeSelector {
		selectorLabels[k] = v
	}

	var resourceRequest corev1.ResourceList
	for k, v := range cr.Spec.Resources {
		if resourceRequest == nil {
			resourceRequest = make(map[corev1.ResourceName]resource.Quantity)
		}

		q := resource.NewQuantity(v, resource.DecimalSI)
		resourceRequest[corev1.ResourceName(k)] = *q
	}

	var ssh bool
	if cr.Spec.SSH != nil {
		ssh = true
	}
	args := []string{
		fmt.Sprintf("--batch=%s", cr.Spec.Batch),
		fmt.Sprintf("--config=%s", slurmCfgPath),
		fmt.Sprintf("--ssh=%t", ssh),
	}

	if cr.Spec.Results != nil {
		args = append(args, fmt.Sprintf("--cr-mount=%s", "/collect"))
		if cr.Spec.Results.From != "" {
			args = append(args, fmt.Sprintf("--file-to-collect=%s", cr.Spec.Results.From))
		}
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-job",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "jt1",
					Image: fmt.Sprintf("%s:%s", imageName, jobCompanionVersion),
					Args:  args,
					Resources: corev1.ResourceRequirements{
						Requests: resourceRequest,
						Limits:   resourceRequest,
					},
					Env:          getEnvs(cr),
					VolumeMounts: getVolumesMount(cr),
				},
			},
			Volumes:       getVolumes(cr),
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
