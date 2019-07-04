# WLM-operator

[![CircleCI](https://circleci.com/gh/sylabs/wlm-operator.svg?style=svg&circle-token=7222176bc78c1ddf7ea4ea615d2e568334e7ec0a)](https://circleci.com/gh/sylabs/wlm-operator)

**WLM operator** is a Kubernetes operator implementation, capable of submitting and
monitoring WLM jobs, while using all of Kubernetes features, such as smart scheduling and volumes.

WLM operator connects Kubernetes node with a whole WLM cluster, which enables multi-cluster scheduling.
In other words, Kubernetes integrates with WLM as one to many.

Each WLM partition(queue) is represented as a dedicated virtual node in Kubernetes. WLM operator
can automatically discover WLM partition resources(CPUs, memory, nodes, wall-time) and propagates them
to Kubernetes by labeling virtual node. Those node labels will be respected during Slurm job scheduling so that a
job will appear only on a suitable partition with enough resources.

Right now WLM-operator supports only SLURM clusters. But it's easy to add a support for another WLM. For it you need to implement a [GRPc server](https://github.com/sylabs/wlm-operator/blob/master/pkg/workload/api/workload.proto). You can use [current SLURM implementation](https://github.com/sylabs/wlm-operator/blob/master/internal/red-box/api/slurm.go) as a reference.

<p align="center">
  <img style="width:100%;" height="600" src="./docs/integration.svg">
</p>

## Installation

### Prerequisites

- Go 1.11+

### Installation steps

Installation process is required to connect Kubernetes with Slurm cluster.

*NOTE*: further described installation process for a single Slurm cluster,
the same steps should be performed for each cluster to be connected.

1. Create a new Kubernetes node with [Singularity-CRI](https://github.com/sylabs/singularity-cri) on the
Slurm login host. Make sure you set up NoSchedule taint so that no random pod will be scheduled there.

2. Create a new dedicated user on the Slurm login host. All submitted Slurm jobs will be executed on behalf
of that user. Make sure the user has execute permissions for the following Slurm binaries:`sbatch`,
`scancel`, `sacct` and `scontol`.

3. Pull the repo.
```bash
go get -d github.com/sylabs/wlm-operator
```

4. Build and start *red-box* – a gRPC proxy between Kubernetes and a Slurm cluster.
```bash
cd $GOPATH/github.com/sylabs/wlm-operator && make
```
Use dedicated user from step 2 to run red-box, e.g. set up `User` in systemd red-box.service.
By default red-box listens on `/var/run/syslurm/red-box.sock`, so you have to make sure the user has
read and write permissions for `/var/run/syslurm`.

5. Set up Slurm operator in Kubernetes.
```bash
kubectl apply -f deploy/crds/slurm_v1alpha1_slurmjob.yaml
kubectl apply -f deploy/operator-rbac.yaml
kubectl apply -f deploy/operator.yaml
```
This will create new [CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) that
introduces `SlurmJob` to Kubernetes. After that, Kubernetes controller for `SlurmJob` CRD is set up as a Deployment.

6. Start up configurator that will bring up a virtual node for each partition in the Slurm cluster.
```bash
kubectl apply -f deploy/configurator.yaml
```

After all those steps Kubernetes cluster is ready to run SlurmJobs. 

## Usage

The most convenient way to submit them is using YAML files, take a look at [basic examples](/examples).

We will walk thought basic example how to submit jobs to Slurm in Vagrant.

```yaml
apiVersion: wlm.sylabs.io/v1alpha1
kind: SlurmJob
metadata:
  name: cow
spec:
  batch: |
    #!/bin/sh
    #SBATCH --nodes=1
    #SBATCH --output cow.out
    srun singularity pull -U library://sylabsed/examples/lolcow
    srun singularity run lolcow_latest.sif
    srun rm lolcow_latest.sif
  nodeSelector:
    wlm.sylabs.io/containers: singularity
  results:
    from: cow.out
    mount:
      name: data
      hostPath:
        path: /home/job-results
        type: DirectoryOrCreate
```

In the example above we will run lolcow Singularity container in Slurm and collect the results 
to `/home/job-results` located on a k8s node where job has been scheduled. Generally, job results
can be collected to any supported [k8s volume](https://kubernetes.io/docs/concepts/storage/volumes/).

Slurm job specification will be processed by operator and a dummy pod will be scheduled in order to transfer job
specification to a specific queue. That dummy pod will not have actual physical process under that hood, but instead 
its specification will be used to schedule slurm job directly on a connected cluster. To collect results another pod
will be created with UID and GID 1000 (default values), so you should make sure it has a write access to 
a volume where you want to store the results (host directory `/home/job-results` in the example above).
The UID and GID are inherited from virtual kubelet that spawns the pod, and virtual kubelet inherits them
from configurator (see `runAsUser` in [configurator.yaml](./deploy/configurator.yaml)).

After that you can submit cow job:
```bash
$ kubectl apply -f examples/cow.yaml 
slurmjob.wlm.sylabs.io "cow" created

$ kubectl get slurmjob
NAME   AGE   STATUS
cow    66s   Succeeded


$ kubectl get pod
NAME                             READY   STATUS         RESTARTS   AGE
cow-job                          0/1     Job finished   0          17s
cow-job-collect                  0/1     Completed      0          9s
```

Validate job results appeared on a node:
```bash
$ ls -la /home/job-results
cow-job
  
$ ls /home/job-results/cow-job 
cow.out

$ cat cow.out
WARNING: No default remote in use, falling back to: https://library.sylabs.io
 _________________________________________
/ It is right that he too should have his \
| little chronicle, his memories, his     |
| reason, and be able to recognize the    |
| good in the bad, the bad in the worst,  |
| and so grow gently old all down the     |
| unchanging days and die one day like    |
| any other day, only shorter.            |
|                                         |
\ -- Samuel Beckett, "Malone Dies"        /
 -----------------------------------------
        \   ^__^
         \  (oo)\_______
            (__)\       )\/\
                ||----w |
                ||     ||
```


### Results collection

Slurm operator supports result collection into [k8s volume](https://kubernetes.io/docs/concepts/storage/volumes/)
so that a user won't need to have access Slurm cluster to analyze job results.

However, some configuration is required for this feature to work. More specifically, results can be collected
located on a login host only (i.e. where `red-box` is running), while Slurm job can be scheduled on an arbitrary
Slurm worker node. This means that some kind of a shared storage among Slurm nodes should be configured so that despite
of a Slurm worker node chosen to run a job, results will appear on a login host as well. 
_NOTE_: result collection is a network and IO consuming task, so collecting large files (e.g. 1Gb result of an
ML job) may not be a great idea.

Let's walk through basic configuration steps. Further assumed that file _cow.out_ from example above
is collected. This file can be found on a Slurm worker node that is executing a job.
More specifically, you'll find it in a folder, from which job was submitted (i.e. `red-box`'s working dir).
Configuration for other results file will differ in shared paths only:

	$RESULTS_DIR = red-box's working directory

Share $RESULTS_DIR among all Slurm nodes, e.g set up nfs share for $RESULTS_DIR.


## Configuring red-box

By default red-box performs automatic resources discovery for all partitions.
However, it's possible to setup available resources for a partition manually with in the config file.
The following resources can be specified: `nodes`, `cpu_per_node`, `mem_per_node` and `wall_time`. 
Additionally you can specify partition features there, e.g. available software or hardware. 
Config path should be passed to red-box with the `--config` flag.

Config example:
```yaml
patition1:
  nodes: 10
  mem_per_node: 2048 # in MBs
  cpu_per_node: 8
  wall_time: 10h 
partition2:
  nodes: 10
  # mem, cpu and wall_time will be automatic discovered
partition3:
  additional_feautres:
    - name: singularity
      version: 3.2.0
    - name: nvidia-gpu
      version: 2080ti-cuda-7.0
      quantity: 20
```


## Vagrant

If you want to try wlm-operator locally before updating your production cluster, use vagrant that will automatically
install and configure all necessary software:

```bash
cd vagrant
vagrnat up && vagrant ssh k8s-master
```
_NOTE_: `vagrant up` may take about 15 minutes to start as k8s cluster will be installed from scratch.

Vagrant will spin up two VMs: a k8s master and a k8s worker node with Slurm installed.
If you wish to set up more workers, fell free to modify `N` parameter in [Vagrantfile](./vagrant/Vagrantfile).
