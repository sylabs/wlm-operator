#!/usr/bin/env bash

GOPATH="${HOME}/go"
SINGULARITY_SLURM_OPERATOR_REPO="github.com/sylabs/slurm-operator"
export PATH=/usr/local/go/bin:${PATH}:${GOPATH}/bin

cd ${GOPATH}/src/${SINGULARITY_SLURM_OPERATOR_REPO} && make

sudo mkdir -p /var/run/syslurm
sudo chown vagrant /var/run/syslurm

export RED_BOX_SERVICE=$(cat <<EOF
[Unit]
Description=Slurm operator red-box
StartLimitIntervalSec=0
[Service]
Type=simple
Restart=always
RestartSec=30
User=vagrant
Group=vagrant
WorkingDirectory=/home/vagrant
ExecStart=/home/vagrant/go/src/github.com/sylabs/slurm-operator/bin/red-box
EOF
)

sudo sh -c "printf "%s" '${RED_BOX_SERVICE}' >> /etc/systemd/system/red-box.service"
sudo systemctl start red-box
systemctl status red-box
