#!/usr/bin/env bash

kubectl apply -f wlm-operator/deploy/crds/wlm_v1alpha1_slurmjob.yaml
kubectl apply -f wlm-operator/deploy/operator-rbac.yaml
kubectl apply -f wlm-operator/deploy/operator.yaml
