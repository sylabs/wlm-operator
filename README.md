# Slurm operator
Singularity implementation of k8s operator for interacting with SLURM.

## Slurm resource daemon

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

