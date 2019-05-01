[![CircleCI](https://circleci.com/gh/sylabs/slurm-operator.svg?style=svg&circle-token=7222176bc78c1ddf7ea4ea615d2e568334e7ec0a)](https://circleci.com/gh/sylabs/slurm-operator)

# Slurm operator
Singularity implementation of k8s operator for interacting with Slurm.

With slurm operator batch jobs can be managed via Kubernetes. To do that operator
will spawn a `job-companion` container that will talk to Slurm.

<p align="center">
  <img width="580" height="400" src="/docs/slurm-k8s.png">
</p>

## Installation

It is assumed you already have Kubernetes with Singularity-CRI and Slurm clusters running in a suitable topology.
There are a couple steps needed to set up slurm operator:

1. set up red-box
2. set up slurm resource daemon on kubernetes cluster 
3. set up slurm operator

### Setting up red-box

Red-box is a REST HTTP server over unix sockets that acts as a proxy between `job-companion` and a
Slurm cluster itself. Under the hood it runs Slurm binaries and returns Slurm response in a convenient form
so that `job-companion` understands it.

Red-box should be run on each Kubernetes node where Slurm jobs may be scheduled. 
Steps for setting up red-box for a single node with installed Go 1.10+ are the following: 

```bash
go get github.com/sylabs/slurm-operator
cd $GOPATH/github.com/sylabs/slurm-operator
make bin/red-box
```
After that you should see `bin/red-box` executable with red-box ready to use.
To see available flags you can do 

```bash
./bin/red-box -h
```

The most simple way to run the red-box:
```bash
./bin/red-box
```

This will create `/var/run/syslurm/red-box.sock` socket and start red-box there.
NOTE: `/var/run/syslurm/red-box.sock` location cannot not be changed at the moment.

### Setting up slurm resource daemon

Resource daemon is used for k8s node labeling and resource provisioning so that
Slurm jobs will be correctly scheduled.

Setting up resource daemon requires a few steps. First of all you should determine
which k8s nodes will be used for Slurm job scheduling. Further assumed that all k8s nodes
are leveraged:

```bash
$ kubectl get no
NAME       STATUS    ROLES     AGE       VERSION
node01     Ready     master    7d        v1.13.2
```

In the example above we have a single node cluster, and node name is `node01`. 

After nodes to configure are determined a configuration should be set up. General configuration
scheme is the following:

	<node1_name>:
	  resources:
	    <resource name>: <quantity>
	  labels:
	    <label name>: <label value>
	<node2_name>:
	  resources:
	    <resource name>: <quantity>
	  labels:
	    <label name>: <label value>
	...

Resources and labels reflect capabilities of a Slurm cluster behind the node. They will be applied 
to the node allowing more precise job scheduling. 

Resource daemon expects full configuration to be available in `slurm` file under the location
specified by `--slurm-config` flag. After startup resource daemon will start watching `slurm` file
contents for changes and dynamically adjust corresponding node's labels and resources.

For convenience we automate this process, so all you need to do is set up
[k8s ConfigMap](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/) named
`slurm-config` with a single field `config` as the following:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: slurm-config
  namespace: default
data:
  config: |
    node01:
      resources:
        cpu: 24
      labels:
        cuda: 10.0
        containers: singularity
```

In the [example above](deploy/slurm-config.yaml) `node01` is configured to talk to a Slurm cluster
that has 24 CPUs and cuda version 10.0 installed, which is also reflected in the config. When jobs
are scheduled configured  Slurm resources and labels are taken into account so that a job is scheduled
on a suitable cluster that is capable of executing it.

First, apply the configuration to the existing k8s cluster:
```bash
$ kubectl apply -f deploy/slurm-config.yaml
configmap "slurm-config" created

$ kubelet describe cm slurm-config
Name:         slurm-config
Namespace:    default
Labels:       <none>
Annotations:  <none>

Data
====
config:
----
node01:
  resources:
    cpu: 24
  labels:
    cuda: 10.0
    containers: singularity

Events:  <none>
```


Start resource daemon:
```bash
$ kubectl apply -f deploy/resource-daemon.yaml
serviceaccount "slurm-rd" created
clusterrole.rbac.authorization.k8s.io "slurm-rd" created
clusterrolebinding.rbac.authorization.k8s.io "slurm-rd" created
daemonset.apps "slurm-rd" created

$ kubectl get daemonset
 NAME       DESIRED   CURRENT   READY     UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
 slurm-rd   1         1         1         1            1           <none>          1m
```

All labels and resources from `slurm-config` are prefixed with `slurm.sylabs.io` and applied to the
node. You can see them by running the following:

```bash
kubectl describe node <node-name>
```

You will see something like this:

```text
Labels:             beta.kubernetes.io/arch=amd64
                    beta.kubernetes.io/os=linux
                    kubernetes.io/hostname=node01
                    node-role.kubernetes.io/master=
                    slurm.sylabs.io/containers=singularity
                    slurm.sylabs.io/cuda=10.0
                    slurm.sylabs.io/workload-manager=slurm
...
Capacity:
 slurm.sylabs.io/cpu:  24
...
```

### Setting up slurm operator

Slurm operator extends Kubernetes with CRD and controller, more information about this concepts can be found
[here](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

Setting up slurm operator is quite easy:
```bash
$ kubectl apply -f  deploy/crds/slurm_v1alpha1_slurmjob.yaml 
customresourcedefinition.apiextensions.k8s.io "slurmjobs.slurm.sylabs.io" created

$ kubectl get crd
NAME                        AGE
slurmjobs.slurm.sylabs.io   1m

$ kubectl apply -f deploy/operator-rbac.yaml 
clusterrolebinding.rbac.authorization.k8s.io "slurm-operator" created
clusterrole.rbac.authorization.k8s.io "slurm-operator" created
serviceaccount "slurm-operator" created

$ kubectl apply -f deploy/operator.yaml 
deployment.apps "slurm-operator" created

$ kubectl get deployment
NAME                 DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
slurm-operator       1         1         1            1           37s
```

That's it! After all those steps you can run Slurm jobs via Kubernetes.

## Usage

After installation k8s cluster will be capable to work with `SlurmJob` resource. The most
convenient way to submit them is using YAML files, take a look at [basic examples](/examples).

We will walk thought basic example how to submit jobs to Slurm in Vagrant.

```yaml
apiVersion: slurm.sylabs.io/v1alpha1
kind: SlurmJob
metadata:
  name: cow
spec:
  batch: |
    #!/bin/sh
    ##SBATCH --nodes=1 --cpus-per-task=1
    srun singularity pull library://sylabsed/examples/lolcow
    srun singularity run lolcow_latest.sif
    srun rm lolcow_latest.sif
  nodeSelector:
    slurm.sylabs.io/containers: singularity
  results:
    mount:
      name: data
      hostPath:
        path: /home/job-results
        type: DirectoryOrCreate
    from: slurm-4.out # can be omitted
```

In the example above we will run lolcow Singularity container in Slurm and collect the results 
to `/home/job-results` located on a k8s node where job has been scheduled. Generally, job results
can be collected to any supported [k8s volume](https://kubernetes.io/docs/concepts/storage/volumes/).

By default `job-companion` will be run with UID and GID 1000, so you should make sure it has a write access to 
a volume where you want to store the results (host directory `/home/job-results` in the example above).
The UID and GID can be configured with operator flags, see [operator.yaml](./deploy/operator.yaml) for the example.

After that you can submit cow job:
```bash
$ kubectl apply -f examples/cow.yaml 
slurmjob.slurm.sylabs.io "cow" created

$ kubectl get slurmjob
NAME      AGE
cow       11s

$ kubectl get pod
NAME                                  READY     STATUS      RESTARTS   AGE
cow-job                               0/1       Completed   0          30m
```

Validate job results appeared on a node:
```bash
$ ls -la /home/job-results
cow-job
  
$ cd /home/job-results/cow-job 
$ ls
slurm-4.out

$ cat slurm-4.out 
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

Slurm operator supports result collection to a provided [k8s volume](https://kubernetes.io/docs/concepts/storage/volumes/)
so that a user won't need to have access to a Slurm cluster to analyze job results.

However, some configuration is required for this feature to work. More specifically, job-companion can collect results
located on a submit server only (i.e. where `red-box` is running), while Slurm job can be scheduled on an arbitrary
Slurm worker node. This means that some kind of a shared storage among Slurm nodes should be configured so that despite
of Slurm worker node chosen to run a job, results will appear on a submit server as well. 
NOTE: result collection is a network and IO consuming task, so collecting large files (e.g. 1Gb result of a
machine learning job) may not be a great idea.

Let's walk through basic configuration steps. Further assumed that default results file
is collected (_slurm-<jobID>.out_). This file can be found on a Slurm worker node that is executing a job.
More specifically, you'll find them in a folder, from which job was submitted (i.e. `red-box`'s working dir).
Configuration for custom results file will differ in shared paths only:

	$RESULTS_DIR = red-box's working directory

Share $RESULTS_DIR among all Slurm nodes, e.g set up nfs share for $RESULTS_DIR.

## Developers

Before submitting any pull requests make sure you have done the following:
1. Updated dependencies in vendor if needed (`make dep`)
2. Checked code is buildable
3. Ran tests and linters (`make test && make lint`)
4. Updated generated files (`make gen`) 


## Vagrant

You can use vagrant with k8s, slurm and slurm-operator for testing purposes.

To start vagrant box:  

```bash
cd vagrant
vagrnat up && vagrant ssh k8s-master
```
`vagrant up` - will take near 15 minutes to start.

Vagrant will spin up tree VMs - a k8s master and two k8s worker nodes. SLURM is installed on each k8s worker. Slurm-operator is also installed and ready to use.
