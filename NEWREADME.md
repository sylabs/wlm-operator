# SLURM operator

*SLURM operator* is an implementation for kubernetes operator which provides a possibility to submit and to track job on a SLURM cluster with the use of kubernetes advantages, such as scheduling and volumes. Kubernetes integrates with slurm as one to mane, one kubernets cluster and many SLURM clusters. Each SLURM partition (quque) is presented in Kubernetes as a dedicated node (virtual-kubelet). *SLURM operator* can automaticaly discover SLURM partition capabilites (CPU, memory, nodes, WallTime) and propogate them to k8s as node labels. Those node labes are used for k8s scheduling. So based on job resources requiremtes can k8s itself select SLURM partition on which to start a job.

<p align="center">
  <img width="640" height="400" src="/docs/login-node-integration.png">
</p>

## Installation

*Installation process is required to have settled Kubernetes and SLURM clusters.*

*The installation process is described for one SLURM cluster, but it applicable to arbitrary number of clusters.*

1. Create a new Kubernetes node with [SyCri](https://github.com/sylabs/singularity-cri) on the same machine with SLURM login host. Add a custom NoSchedule Taint to the created node, to be sure no random pods be allocated on the node.
2. On the SLURM login host setup a new UNIX user. All SLURM jobs will be submitted from that user. Be sure SLURM commands: `sbatch, scancel, sacct, scontol show jobid, scontrol show partition  ` are accessible from the created user.
3. Next is required to build *red-box* binary, *red-box* is a gRPC proxy between Kubernetes *SlurmJob* and SLURM cluster. For building red-box Go 1.10+ is required.
   1. `go get github.com/sylabs/slurm-operator`
   2. `cd $GOPATH/github.com/sylabs/slurm-operator`
   3. `make bin/red-box`
   4. *optional* create a systemd service for *red-box*.
4. SLURM operator setup.
   1. `cd $GOPATH/github.com/sylabs/slurm-operator`
   2. Create SlurmJob [CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) `kubectl apply -f deploy/crds/slurm_v1alpha1_slurmjob.yaml`
   3. Create a new ServiceAccount with permissions for operator `kubectl apply -f deploy/operator-rbac.yaml `
   4. Start controller `kubectl apply -f deploy/operator.yaml `
5. Create virtual nodes for SLURM partitions.
   1. `cd $GOPATH/github.com/sylabs/slurm-operator`
   2. Start DaemonSet which will create virtual-kubelet pods `kubectl apply -f deploy/configurator.yaml`

At this point Kubernetes cluster is ready to run SlurmJobs. 
