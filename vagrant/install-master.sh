#!/usr/bin/env bash

sudo kubeadm init --cri-socket="/var/run/singularity.sock" --ignore-preflight-errors=all --apiserver-advertise-address="192.168.60.10" --apiserver-cert-extra-sans="192.168.60.10"  --node-name k8s-master --pod-network-cidr=10.244.0.0/16

mkdir -p /home/vagrant/.kube
sudo cp -i /etc/kubernetes/admin.conf /home/vagrant/.kube/config
sudo chown vagrant:vagrant /home/vagrant/.kube/config

IPADDR=`ifconfig eth1 | grep inet | awk '{print $2}'| cut -f2 -d:`
echo 'Environment="KUBELET_EXTRA_ARGS=--node-ip='${IPADDR}'"' | sudo tee -a /etc/systemd/system/kubelet.service.d/10-kubeadm.conf

kubectl apply -f /sync/etc/flannel.yaml

sudo systemctl daemon-reload
sudo systemctl restart kubelet

export JOIN_COMMAND=$(kubeadm token create --print-join-command)
printf "%s --ignore-preflight-errors=all --cri-socket=\"/var/run/singularity.sock\"\n" "${JOIN_COMMAND}" > /sync/etc/join.sh
