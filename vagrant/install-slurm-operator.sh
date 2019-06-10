#!/usr/bin/env bash

GOPATH="${HOME}/go"
SINGULARITY_SLURM_OPERATOR_REPO="github.com/sylabs/slurm-operator"
export PATH=/usr/local/go/bin:${PATH}:${GOPATH}/bin

cd ${GOPATH}/src/${SINGULARITY_SLURM_OPERATOR_REPO} && make
cat > ${HOME}/config.yaml <<EOF
debug:
  auto_nodes: true
EOF

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
WorkingDirectory=${HOME}
ExecStart=${GOPATH}/src/${SINGULARITY_SLURM_OPERATOR_REPO}/bin/red-box
EOF
)

sudo sh -c "printf "%s\n" '${RED_BOX_SERVICE}' >> /etc/systemd/system/red-box.service"
sudo systemctl start red-box
systemctl status red-box
