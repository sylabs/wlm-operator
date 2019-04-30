#!/usr/bin/env bash

sudo chmod +x /sync/etc/join.sh 
sudo /sync/etc/join.sh

IPADDR=`ifconfig eth1 | grep inet | awk '{print $2}'| cut -f2 -d:`
echo 'Environment="KUBELET_EXTRA_ARGS=--node-ip='${IPADDR}'"' | sudo tee -a /etc/systemd/system/kubelet.service.d/10-kubeadm.conf

sudo systemctl daemon-reload
sudo systemctl restart kubelet
