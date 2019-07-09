#!/usr/bin/env bash

sudo chmod +x /sync/etc/join.sh 
sudo /sync/etc/join.sh
cp /sync/etc/config .kube

expot IPADDR=$(ifconfig eth1 | grep inet | awk '{print $2}'| cut -f2 -d:)
sudo -E sh -c 'cat > /test.conf <<EOF
Environment="KUBELET_EXTRA_ARGS=--node-ip=${IPADDR}"
EOF'

sudo systemctl daemon-reload
sudo systemctl restart kubelet
