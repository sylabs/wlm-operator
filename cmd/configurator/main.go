package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/pkg/workload/api"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

var (
	redBoxSock = flag.String("sock", "", "path to red-box socket")

	serviceAccount = os.Getenv("SERVICE_ACCOUNT")
	kubeletImage   = os.Getenv("KUBELET_IMAGE")
	hostNodeName   = os.Getenv("HOST_NAME")
	namespace      = os.Getenv("NAMESPACE")

	uid = int64(os.Geteuid())
	gid = int64(os.Getgid())
)

func main() {
	flag.Parse()

	if namespace == "" {
		namespace = "default"
	}

	// getting k8s config.
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("can't fetch cluster config %s", err)
	}

	// corev1 client set is required to create collecting results pod.
	coreC, err := corev1.NewForConfig(config)
	if err != nil {
		log.Fatalf("can't create core client %s", err)
	}

	conn, err := grpc.Dial("unix://"+*redBoxSock, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("can't connect to %s %s", *redBoxSock, err)
	}
	slurmC := api.NewWorkloadManagerClient(conn)

	ctx, cancel := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go watchPartitions(ctx, wg, slurmC, coreC)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, unix.SIGINT, unix.SIGTERM, unix.SIGQUIT)

	log.Printf("Got signal %s", <-sig)
	cancel()

	wg.Wait()

	log.Println("Configurator is finished")
}

func watchPartitions(ctx context.Context, wg *sync.WaitGroup,
	slurm api.WorkloadManagerClient, k8s *corev1.CoreV1Client) {

	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.Tick(1 * time.Minute):
			// getting SLURM partitions
			partitionsResp, err := slurm.Partitions(context.Background(), &api.PartitionsRequest{})
			if err != nil {
				log.Printf("Can't get partitions %s", err)
				continue
			}

			// gettings k8s nodes
			nodes, err := k8s.Nodes().List(metav1.ListOptions{
				LabelSelector: "type=virtual-kubelet",
			})
			if err != nil {
				log.Printf("Can't get virtual nodes %s", err)
				continue
			}
			// extract partition names from k8s nodes
			nNames := partitionNames(nodes.Items)

			// check which partitions are not yet represented in k8s
			partitionToCreate := notIn(partitionsResp.Partition, nNames)
			// creating pods for that partitions
			if err := createNodeForPartitions(k8s, partitionToCreate); err != nil {
				log.Printf("Can't create partitions  %s", err)
			}

			// some partitions can be deleted from SLURM, so we need to delete pods
			// which represent those deleted partitions
			nodesToDelete := notIn(nNames, partitionsResp.Partition)
			if err := deleteControllingPod(k8s, nodesToDelete); err != nil {
				log.Printf("Can't delete controlling pod %s", err)
			}
		}
	}
}

func createNodeForPartitions(podsGetter corev1.PodsGetter, partitions []string) error {
	for _, p := range partitions {
		log.Printf("Creating pod for %s partition in %s namespace", p, namespace)
		_, err := podsGetter.Pods(namespace).Create(virtualKubeletPodTemplate(p, hostNodeName))
		if err != nil {
			return errors.Wrapf(err, "could not create pod for %s partition", p)
		}
	}

	return nil
}

func deleteControllingPod(podsGetter corev1.PodsGetter, nodes []string) error {
	for _, n := range nodes {
		nodeName := partitionNodeName(n, hostNodeName)
		log.Printf("Deleting pod %s in %s namespace", nodeName, namespace)
		err := podsGetter.Pods(namespace).Delete(nodeName, &metav1.DeleteOptions{})
		if err != nil {
			return errors.Wrapf(err, "could not delete pod %s", nodeName)
		}
	}
	return nil
}

// virtualKubeletPodTemplate returns filled pod model ready to be created in k8s.
// Kubelet pod will create virtual node that will be responsible for handling Slurm jobs.
func virtualKubeletPodTemplate(partitionName, nodeName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: partitionNodeName(partitionName, nodeName),
		},
		Spec: v1.PodSpec{
			ServiceAccountName: serviceAccount,
			Containers: []v1.Container{
				{
					Name:            "vk",
					Image:           kubeletImage,
					ImagePullPolicy: v1.PullAlways,
					Args: []string{
						"--nodename",
						partitionNodeName(partitionName, nodeName),
						"--provider",
						"slurm",
						"--startup-timeout",
						"10s",
					},
					Ports: []v1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 10255,
						},
					},
					ReadinessProbe: &v1.Probe{
						Handler: v1.Handler{
							HTTPGet: &v1.HTTPGetAction{
								Path: "/stats/summary",
								Port: intstr.IntOrString{
									Type:   intstr.String,
									StrVal: "metrics",
								},
							},
						},
					},
					Env: []v1.EnvVar{
						{
							Name: "VK_HOST_NAME",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
						{
							Name: "VK_POD_NAME",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "metadata.name",
								},
							},
						},
						{
							Name: "VKUBELET_POD_IP",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "status.podIP",
								},
							},
						},
						{
							Name:  "PARTITION",
							Value: partitionName,
						},
						{
							Name:  "RED_BOX_SOCK",
							Value: *redBoxSock,
						},
						{
							Name:  "APISERVER_CERT_LOCATION",
							Value: "/kubelet.crt",
						},
						{
							Name:  "APISERVER_KEY_LOCATION",
							Value: "/kubelet.key",
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "syslurm-mount",
							MountPath: "/syslurm",
						},
						{
							Name:      "kubelet-crt",
							MountPath: "/kubelet.crt",
						},
						{
							Name:      "kubelet-key",
							MountPath: "/kubelet.key",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "syslurm-mount", // directory with red-box socket
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/run/syslurm",
							Type: &[]v1.HostPathType{v1.HostPathDirectory}[0],
						},
					},
				},
				{
					Name: "kubelet-crt", // we need certificates for pod rest api, k8s gets pods logs via rest api
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/lib/kubelet/pki/kubelet.crt",
							Type: &[]v1.HostPathType{v1.HostPathFile}[0],
						},
					},
				},
				{
					Name: "kubelet-key",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/lib/kubelet/pki/kubelet.key",
							Type: &[]v1.HostPathType{v1.HostPathFile}[0],
						},
					},
				},
			},
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:  &uid,
				RunAsGroup: &gid,
			},
		},
	}
}

// partitionNames extracts slurm partition name from k8s node labels
func partitionNames(nodes []v1.Node) []string {
	names := make([]string, 0)
	for _, n := range nodes {
		if l, ok := n.Labels["slurm.sylabs.io/partition"]; ok {
			names = append(names, l)
		}
	}

	return names
}

// notIn returns elements from s1 which are not in s2
func notIn(s1, s2 []string) []string {
	notIn := make([]string, 0)
	for _, e1 := range s1 {
		if contains(s2, e1) {
			continue
		}
		notIn = append(notIn, e1)
	}

	return notIn
}

// contains checks if elements presents in s
func contains(s []string, e string) bool {
	for _, e1 := range s {
		if e == e1 {
			return true
		}
	}

	return false
}

// partitionNodeName forms partition name that will be used as pod and node name in k8s
func partitionNodeName(partition, node string) string {
	return fmt.Sprintf("slurm-%s-%s", node, partition)
}
