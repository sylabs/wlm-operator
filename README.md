# Slurm operator
Singularity implementation of k8s operator for interacting with Slurm.

With slurm operator batch jobs can be managed via Kubernetes. To do that operator
will spawn a `job-companion` container that will talk to slurm. There are two slurm
connection modes supported: local communication or ssh connection. The difference of
that two modes and what cluster topology they require is illustrated below.

![local mode](/docs/local-mode.png =500x500)
![ssh mode](/docs/ssh-mode.png =500x500)

Depending on the chosen mode installation steps may vary.

## Installation

It is assumed you already have Kubernetes and Slurm clusters running in a topology
suitable for the chosen connection mode. There are a couple steps needed to set up slurm operator:

1. set up slurm controller (for local connection mode only)
2. set up slurm resource daemon on kubernetes cluster 
3. set up slurm operator

### Setting up slurm controller

Slurm controller is a REST HTTP server that acts as a proxy between `job-companion` and
Slurm itself. Under the hood it runs Slurm binaries and returns Slurm response in a convenient form
so that `job-companion` understands it.

Slurm controller should be run on each Kubernetes node where slurm jobs may be scheduled. 
Steps for setting up slurm controller for a single node with installed Go 1.10+ are the following: 

```bash
go get github.com/sylabs/slurm-operator
cd $GOPATH/github.com/sylabs/slurm-operator
make bin/slurm-controller
```
After that you should see `bin/slurm-controller` executable with controller ready to use.
To see available flags you can do 

```bash
./bin/slurm-controller -h
```

The most simple way to run the controller:
```bash
./bin/slurm-controller 
``` 

For production purposes you may want to set up controller as a service, but that topic
will not be covered here.

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
      slurm: <address of slurm to connect to via ssh OR address of slurm controller>
      resources:
        <resource name>: <quantity>
      labels:
        <label name>: <label value>
	<node2_name>:
      slurm: <address of slurm to connect to via ssh OR address of slurm controller>
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
      slurm: my.slurm.ssh:2222
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
  slurm: my.slurm.ssh:2222
  resources:
    cpu: 2
  labels:
    cuda: 10.0

Events:  <none>
```

By default resource daemon will be run with uid 1000, so you should make sure it has write access to
`/var/lib/syslurm` directory where configuration for `job-companion` will be stored.

Start resource daemon:
```bash
$ kubectl apply -f deploy/resource-daemon.yaml
serviceaccount "slurm-resource-daemon" created
clusterrole.rbac.authorization.k8s.io "slurm-resource-role" created
clusterrolebinding.rbac.authorization.k8s.io "slurm-resource-bind" created
daemonset.apps "slurm-rd" created

$ kubectl get ds
 NAME       DESIRED   CURRENT   READY     UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
 slurm-rd   1         1         1         1            1           <none>          1m
```

All labels and resources from `slurm-config` are prefixed with `slurm.sylabs.io` and applied to the
node. You can see them by running the following:

```bash
kubectl describe no <node-name>
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

Also on node you should see `slurm-cfg.yaml` file created in `/var/lib/syslurm`.

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

$ kubectl get deploy
NAME                 DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
slurm-job-operator   1         1         1            1           37s
```

That's it! After all those steps you can run Slurm jobs via Kubernetes.

## Usage

After installation k8s cluster will be capable to work with `SlurmJob` resource. The most
convenient way to submit them is using YAML files, take a look at [basic examples](/examples).

We will walk thought basic example that uses ssh to submit jobs to Slurm in Vagrant.

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
  ssh:
    user: vagrant
    key:
      secretKeyRef:
        namek get : slurm
        key: key
```

In the example above we will run lolcow Singularity container in Slurm and collect the results 
to `/home/vagrant/job-results` located on node.

Before submitting this cow job you'll need to configure `slurm` config map and store Vagrant ssh
key there as follows:

```bash
$ vagrant ssh-config
Host default
  HostName 127.0.0.1
  User vagrant
  Port 2222
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  IdentityFile /Users/sasha/.vagrant/machines/default/virtualbox/private_key
  IdentitiesOnly yes
  LogLevel FATAL


$ kubectl create secret generic  slurm --from-file=key=/Users/sasha/slurm/.vagrant/machines/default/virtualbox/private_key
secret "slurm" created
```

By default `job-companion` will be run with uid 1000, so you should make sure it has write access to
a directory where you want to store the results (`/home/vagrant/job-results` in the example above).

After that you can submit cow job:

```bash
$ kubectl apply -f examples/cow-ssh.yaml 
slurmjob.slurm.sylabs.io "cow" created

$ kubectl get slurmjob
NAME      AGE
cow       11s

$ kubectl get po
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
