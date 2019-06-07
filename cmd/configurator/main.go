package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/util/intstr"

	"golang.org/x/sys/unix"

	"google.golang.org/grpc"

	"github.com/sylabs/slurm-operator/pkg/workload/api"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

var (
	redBoxSock = flag.String("sock", "", "path to red-box socket")

	serviceAccount = os.Getenv("SERVICE_ACCOUNT")
	kubeletImage   = os.Getenv("KUBELET_IMAGE")
	hostNodeName   = os.Getenv("HOST_NAME")
)

func main() {
	flag.Parse()

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

	go watchPartitions(ctx, slurmC, coreC)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, unix.SIGINT, unix.SIGTERM, unix.SIGQUIT)

	log.Printf("Got signal %s", <-sig)
	cancel()
}

func watchPartitions(ctx context.Context, slurmClient api.WorkloadManagerClient, k8sClient *corev1.CoreV1Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.Tick(30 * time.Second):
			partitionsResp, err := slurmClient.Partitions(context.Background(), &api.PartitionsRequest{})
			if err != nil {
				log.Printf("Can't get partitions %s", err)
				continue
			}

			nodes, err := k8sClient.Nodes().List(metav1.ListOptions{
				LabelSelector: "type=virtual-kubelet",
			})
			if err != nil {
				log.Printf("Can't get pods %s", err)
			}

			nNames := partitionNames(nodes.Items)

			partitionToCreate := notIn(partitionsResp.Partition, nNames)
			if err := createNodeForPartitions(k8sClient, partitionToCreate); err != nil {
				log.Printf("Can't create partitions  %s", err)
			}

			nodesToDelete := notIn(nNames, partitionsResp.Partition)
			if err := deleteControllingPod(k8sClient, nodesToDelete); err != nil {
				log.Printf("Can't mark node as dead %s", err)
			}
		}

	}
}

func createNodeForPartitions(k8sClient *corev1.CoreV1Client, partitions []string) error {
	for _, p := range partitions {
		_, err := k8sClient.Pods("default").
			Create(virtualKubeletPodTemplate(p, hostNodeName))
		if err != nil {
			return errors.Wrap(err, "can't create pod for partition")
		}
	}

	return nil
}

func deleteControllingPod(k8sClient *corev1.CoreV1Client, nodes []string) error {
	for _, n := range nodes {
		nodeName := partitionNodeName(n, hostNodeName)
		if err := k8sClient.Pods("default").Delete(nodeName, &metav1.DeleteOptions{}); err != nil {
			return errors.Wrap(err, "can't delete pod")
		}
	}
	return nil
}

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
					Name: "syslurm-mount",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/run/syslurm",
							Type: &[]v1.HostPathType{v1.HostPathDirectory}[0],
						},
					},
				},
				{
					Name: "kubelet-crt",
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
		},
	}
}

func partitionNames(nodes []v1.Node) []string {
	names := make([]string, 0, 0)
	for _, n := range nodes {
		if l, ok := n.Labels["slurm.sylabs.io/partition"]; ok {
			names = append(names, l)
		}
	}

	return names
}

func notIn(s1, s2 []string) []string {
	notIn := make([]string, 0, 0)
	for _, e1 := range s1 {
		if contains(e1, s2) {
			continue
		}
		notIn = append(notIn, e1)
	}

	return notIn
}

func contains(e string, s []string) bool {
	for _, e1 := range s {
		if e == e1 {
			return true
		}
	}

	return false
}

func partitionNodeName(partition, node string) string {
	return fmt.Sprintf("slurm-%s-%s", node, partition)
}
