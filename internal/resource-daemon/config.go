package resource_daemon

// NodeConfig contains SLURM cluster local/ssh address.
// that config is used for file which resource-daemon creates on each k8s node
// job-companion uses that addresses from file for commenting with slurm
type NodeConfig struct {
	SSHAddr   string `yaml:"ssh_addr"`
	LocalAddr string `yaml:"local_add"`
}
