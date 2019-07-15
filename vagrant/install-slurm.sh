#!/usr/bin/env bash

export DEBIAN_FRONTEND=noninteractive
sudo -E apt-get install -y munge
sudo -E apt-get install -y slurm-wlm slurm-wlm-basic-plugins

HOST_NAME=$(hostname)
export HOST_NAME

sudo -E sh -c 'cat > /etc/slurm-llnl/slurm.conf <<EOF
ControlMachine=${HOST_NAME}
AuthType=auth/munge
CacheGroups=0
CryptoType=crypto/munge
MpiDefault=none
ProctrackType=proctrack/pgid
ReturnToService=1
SlurmctldPidFile=/var/run/slurm-llnl/slurmctld.pid
SlurmctldPort=6817
SlurmdPidFile=/var/run/slurm-llnl/slurmd.pid
SlurmdPort=6818
SlurmdSpoolDir=/var/lib/slurm-llnl/slurmd
SlurmUser=vagrant
StateSaveLocation=/var/lib/slurm-llnl/slurmctld
SwitchType=switch/none
TaskPlugin=task/none
InactiveLimit=0
KillWait=30
MinJobAge=300
SlurmctldTimeout=120
SlurmdTimeout=300
Waittime=0
FastSchedule=1
SchedulerType=sched/backfill
SchedulerPort=7321
SelectType=select/linear
AccountingStorageType=accounting_storage/filetxt
AccountingStorageLoc=/var/log/slurm-llnl/accounting
AccountingStoreJobComment=YES
ClusterName=cluster
JobCompType=jobcomp/filetxt
JobAcctGatherFrequency=30
JobAcctGatherType=jobacct_gather/none
JobCompLoc=/var/log/slurm-llnl/job_completions
SlurmctldDebug=3
SlurmctldLogFile=/var/log/slurm-llnl/slurmctld.log
SlurmdDebug=3
SlurmdLogFile=/var/log/slurm-llnl/slurmd.log
NodeName=${HOST_NAME} CPUs=2 State=UNKNOWN
PartitionName=debug Nodes=${HOST_NAME} Default=YES MaxTime=30 State=UP MaxMemPerNode=512 MaxCPUsPerNode=2 MaxNodes=1
EOF'

sudo chown vagrant /var/log/slurm-llnl
sudo chown vagrant /var/lib/slurm-llnl/slurmctld
sudo chown vagrant /var/run/slurm-llnl
sudo touch /var/log/slurm-llnl/accounting
sudo chown vagrant /var/log/slurm-llnl/accounting

sudo /etc/init.d/slurmctld start
sudo /etc/init.d/slurmd start
