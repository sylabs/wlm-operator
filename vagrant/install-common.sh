#!/usr/bin/env bash

export NETWORK_CONFIG=$(cat <<EOF
{
    "cniVersion": "0.3.1",
    "name": "bridge",
    "plugins": [
        {
            "type": "loopback"
        },
        {
            "type": "bridge",
            "bridge": "cbr0",
            "isGateway": true,
            "isDefaultGateway": true,
            "ipMasq": true,
            "capabilities": {"ipRanges": true},
            "ipam": {
                "type": "host-local",
                "routes": [
                    { "dst": "0.0.0.0/0" }
                ]
            }
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true},
            "snat": true
        }
    ]
}
EOF
)

sudo mkdir -p /etc/cni/net.d
sudo sh -c "printf "%s" '${NETWORK_CONFIG}' >> /etc/cni/net.d/11_bridge.conflist"

export SYCRI_SERVICE=$(cat <<EOF
[Unit]
Description=Singularity-CRI daemon
StartLimitIntervalSec=0
[Service]
Type=simple
Restart=always
RestartSec=30
User=root
Group=root
ExecStart=/home/vagrant/go/src/github.com/sylabs/singularity-cri/bin/sycri
EOF
)


SINGULARITY_REPO="github.com/sylabs/singularity"
SINGULARITY_CRI_REPO="github.com/sylabs/singularity-cri"
SINGULARITY_SLURM_OPERATOR_REPO="github.com/sylabs/slurm-operator"
GOPATH="${HOME}/go"

export DEBIAN_FRONTEND=noninteractive


sudo apt-get update
sudo apt-get install -y build-essential libssl-dev uuid-dev libgpgme11-dev libseccomp-dev pkg-config squashfs-tools

export VERSION=1.12 OS=linux ARCH=amd64

wget -q -O /tmp/go${VERSION}.${OS}-${ARCH}.tar.gz https://dl.google.com/go/go${VERSION}.${OS}-${ARCH}.tar.gz
sudo tar -C /usr/local -xzf /tmp/go${VERSION}.${OS}-${ARCH}.tar.gz

echo 'export GOPATH=${HOME}/go' >> ~/.bashrc && \
echo 'export PATH=/usr/local/go/bin:${PATH}:${GOPATH}/bin' >> ~/.bashrc && \

mkdir ${HOME}/go

export PATH=/usr/local/go/bin:${PATH}:${GOPATH}/bin

go get ${SINGULARITY_REPO}
cd ${GOPATH}/src/${SINGULARITY_REPO} && ./mconfig && cd ./builddir &&  make && sudo make install

go get ${SINGULARITY_CRI_REPO}
cd ${GOPATH}/src/${SINGULARITY_CRI_REPO} && make && sudo make install

go get ${SINGULARITY_SLURM_OPERATOR_REPO}


sudo sh -c "printf "%s" '${SYCRI_SERVICE}' >> /etc/systemd/system/sycri.service"
sudo systemctl start sycri
sudo systemctl status sycri


sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab


curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add
sudo apt-add-repository "deb http://apt.kubernetes.io/ kubernetes-xenial main"
sudo apt-get install -y kubelet kubeadm kubectl

sudo modprobe br_netfilter
sudo sysctl -w net.bridge.bridge-nf-call-iptables=1
sudo sysctl -w net.ipv4.ip_forward=1

sudo mkdir -p /run/flannel
