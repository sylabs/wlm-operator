#!/usr/bin/env bash

cd wlm-operator
kubectl apply -f deploy/crds/wlm_v1alpha1_slurmjob.yaml
kubectl apply -f deploy/operator-rbac.yaml
kubectl apply -f deploy/operator.yaml
