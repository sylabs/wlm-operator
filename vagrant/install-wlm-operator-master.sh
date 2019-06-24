#!/usr/bin/env bash

GOPATH="${HOME}/go"
SINGULARITY_WLM_OPERATOR_REPO="github.com/sylabs/wlm-operator"

kubectl apply -f ${GOPATH}/src/${SINGULARITY_WLM_OPERATOR_REPO}/deploy/crds/wlm_v1alpha1_slurmjob.yaml
kubectl apply -f ${GOPATH}/src/${SINGULARITY_WLM_OPERATOR_REPO}/deploy/operator-rbac.yaml
kubectl apply -f ${GOPATH}/src/${SINGULARITY_WLM_OPERATOR_REPO}/deploy/operator.yaml
