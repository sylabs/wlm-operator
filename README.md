[![CircleCI](https://circleci.com/gh/sylabs/slurm-operator.svg?style=svg&circle-token=7222176bc78c1ddf7ea4ea615d2e568334e7ec0a)](https://circleci.com/gh/sylabs/slurm-operator)

# Slurm operator
Singularity implementation of k8s operator for interacting with Slurm.

With slurm operator batch jobs can be managed via Kubernetes. To do that operator
will spawn a `job-companion` container that will talk to slurm.

<p align="center">
  <img width="460" height="300" src="/docs/slurm-k8s.png">
</p>

## Installation

It is assumed you already have Kubernetes and Slurm clusters running in a suitable topology.
There are a couple steps needed to set up slurm operator:

1. set up red-box
2. set up slurm resource daemon on kubernetes cluster 
3. set up slurm operator

### Setting up red-box

Red-box is a REST HTTP server over unix sockets that acts as a proxy between `job-companion` and
Slurm itself. Under the hood it runs Slurm binaries and returns Slurm response in a convenient form
so that `job-companion` understands it.

Red-box should be run on each Kubernetes node where slurm jobs may be scheduled. 
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
Slurm jobs will be correctly scheduled. It is also responsible for provisioning
`job-companion` with correct slurm address. 

Setting up resource daemon required a few steps. First of all you should determine
which k8s nodes will be used for Slurm job scheduling. Further assumed that all k8s nodes
are leveraged:

```bash
$ kubectl get no
NAME       STATUS    ROLES     AGE       VERSION
minikube   Ready     master    7d        v1.13.2
```

In the example above we have a single node cluster, and node name is `minikube`. 

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

Resources and labels reflect Slurm cluster capabilities behind the node. They will be applied 
to the node allowing more precise job scheduling. 

Resource daemon expects full configuration to be passed in `SLURM_CLUSTER_CONFIG` environment variable.
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
    minikube:
      resources:
        cpu: 2
      labels:
        cuda: 10.0
        containers: singularity
```

In the [example above](deploy/slurm-config.yaml) `minikube` node is configured to talk to Slurm cluster that
is set up in a Vagrant box. Connected Slurm cluster has 2 CPUs and cuda of
version 10.0 installed, which is also reflected in the config. When jobs will be scheduled Slurm resourced
and labels will be taken into account so that job will be scheduled on a cluster that is capable of running it.

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
minikube:
  red_box_addr: /home/vagrant/red-box.sock
  resources:
    cpu: 2
  labels:
    cuda: 10.0

Events:  <none>
```


Start resource daemon:
```bash
$ kubectl apply -f deploy/resource-daemon.yaml
serviceaccount "slurm-resource-daemon" created
clusterrole.rbac.authorization.k8s.io "slurm-resource-role" created
clusterrolebinding.rbac.authorization.k8s.io "slurm-resource-bind" created
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
                    kubernetes.io/hostname=minikube
                    node-role.kubernetes.io/master=
                    slurm.sylabs.io/containers=singularity
                    slurm.sylabs.io/cuda=10.0
                    slurm.sylabs.io/workload-manager=slurm
...
Capacity:
 slurm.sylabs.io/cpu:  2
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
clusterrolebinding.rbac.authorization.k8s.io "slurm-job-operator" created
clusterrole.rbac.authorization.k8s.io "slurm-job-operator" created
serviceaccount "slurm-job-operator" created

$ kubectl apply -f deploy/operator.yaml 
deployment.apps "slurm-job-operator" created

$ kubectl get deployment
NAME                 DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
slurm-job-operator   1         1         1            1           37s
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
        path: /home/vagrant/job-results
        type: DirectoryOrCreate
    from: slurm-4.out # can be omitted
```

In the example above we will run lolcow Singularity container in Slurm and collect the results 
to `/home/vagrant/job-results` located on node. Generally job results can be collected to any
supported [k8s volume](https://kubernetes.io/docs/concepts/storage/volumes/).

By default `job-companion` will be run with uid 1000, so you should make sure it has a write access to 
a volume where you want to store the results (host directory `/home/vagrant/job-results` in the example above).

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

Validate job results appeared on node:
```bash
$ ls -la /home/vagrant/job-results
cow-job
  
$ cd /home/vagrant/job-results/cow-job 
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
located on submit server only (i.e. where `red-box` is running), while slurm job can be scheduled on arbitrary
slurm worker node. It means that some kind of a shared storage among slurm nodes should be configured so that despite
of slurm worker node chosen to run a job, results will appear on submit server as well. 

Let's walk through basic configuration steps. Further assumed that default results file
is collected (_slurm-<jobID>.out_). This file can be found on Slurm worker node that is executing a job in a folder,
from which job was submitted. Configuration for custom results file will differ in shared paths only:

	$RESULTS_DIR = red-box's working directory

Share $RESULTS_DIR among all Slurm nodes, e.g set up nfs share for $RESULTS_DIR.


## Developers

Before submitting any pull requests make sure you have done the following:
1. Updated dependencies in vendor if needed (`make dep`)
2. Checked code is buildable
3. Ran tests and linters (`make test && make lint`)
4. Updated generated files (`make gen`) 
