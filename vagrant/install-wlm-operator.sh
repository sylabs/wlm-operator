#!/usr/bin/env bash

export GOPATH=${HOME}/go
export PATH=${PATH}:/usr/local/go/bin:${GOPATH}/bin

make -C wlm-operator
cat > ${HOME}/config.yaml <<EOF
debug:
  auto_nodes: true
EOF

sudo mkdir -p /var/run/syslurm
sudo chown vagrant /var/run/syslurm

sudo sh -c 'cat  > /etc/systemd/system/red-box.service <<EOF
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
ExecStart=${HOME}/wlm-operator/bin/red-box
EOF'

sudo systemctl start red-box
sudo systemctl status red-box
