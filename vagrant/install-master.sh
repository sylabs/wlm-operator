#!/usr/bin/env bash

sudo kubeadm init --cri-socket="unix:///var/run/singularity.sock" \
          --ignore-preflight-errors=all \
          --apiserver-advertise-address="192.168.60.10" \
          --apiserver-cert-extra-sans="192.168.60.10"  \
          --node-name k8s-master --pod-network-cidr=10.244.0.0/16

mkdir -p /home/vagrant/.kube
sudo cp -i /etc/kubernetes/admin.conf /home/vagrant/.kube/config
sudo chown vagrant:vagrant /home/vagrant/.kube/config
cp /home/vagrant/.kube/config /sync/etc/config

export IPADDR=$(ifconfig eth1 | grep inet | awk '{print $2}'| cut -f2 -d:)
sudo -E sh -c 'cat > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf <<EOF
Environment="KUBELET_EXTRA_ARGS=--node-ip=${IPADDR}"
EOF'

kubectl apply -f /sync/etc/flannel.yaml

sudo systemctl daemon-reload
sudo systemctl restart kubelet

JOIN_COMMAND=$(kubeadm token create --print-join-command)
cat > /sync/etc/join.sh <<EOF
${JOIN_COMMAND} --ignore-preflight-errors=all --cri-socket='unix:///var/run/singularity.sock'
EOF
