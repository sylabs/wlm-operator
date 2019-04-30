#!/usr/bin/env bash

GOPATH="${HOME}/go"
SINGULARITY_SLURM_OPERATOR_REPO="github.com/sylabs/slurm-operator"

kubectl apply -f /sync/etc/slurm-operator-cfg.yaml

kubectl apply -f ${GOPATH}/src/${SINGULARITY_SLURM_OPERATOR_REPO}/deploy/resource-daemon.yaml
kubectl apply -f ${GOPATH}/src/${SINGULARITY_SLURM_OPERATOR_REPO}/deploy/crds/slurm_v1alpha1_slurmjob.yaml
kubectl apply -f ${GOPATH}/src/${SINGULARITY_SLURM_OPERATOR_REPO}/deploy/operator-rbac.yaml
kubectl apply -f ${GOPATH}/src/${SINGULARITY_SLURM_OPERATOR_REPO}/deploy/operator.yaml
