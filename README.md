# Slurm operator
Singularity implementation of k8s operator for interacting with Slurm.

With slurm operator batch jobs can be managed via Kubernetes. To do that operator
will spawn a `job-companion` container that will talk to slurm. There are two slurm
connection modes supported: local communication or ssh connection. The difference of
that two modes and what cluster topology they require is illustrated below.

![ssh mode](/docs/ssh-mode.png)
![local mode](/docs/local-mode.png)

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
Steps for setting up slurm controller for a single node with installed Go are the following: 

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

Works as daemon set inside k8s cluster.

It is responsible for configuring SLURM master address to connect to.

It also configures node labels and resources according values taken from `slurm-config` k8s ConfigMap.

All labels and resources from `slurm-config` are prefixed with `slurm.sylabs.io` and can be viewed
by running the following:

```bash
kubectl describe no <node-name>
``` 

You will see something like this:

```text
Labels:             slurm.sylabs.io/cuda=10.0
                    slurm.sylabs.io/containers=singularity
                    slurm.sylabs.io/infiniband=yes
```

### Installation

To start resource daemon run the following:
 
```bash
kubectl apply -f resource-daemon/deploy/resource-daemon.yaml
``` 

