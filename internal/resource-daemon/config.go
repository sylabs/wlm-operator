package resource_daemon

// NodeConfig contains SLURM cluster local/ssh address.
// NodeConfig is used for the file which resource-daemon creates on each k8s node.
// Job-companion uses addresses from the file for communicating with SLURM cluster.
type NodeConfig struct {
	SSHAddr   string `yaml:"ssh_addr"`
	LocalAddr string `yaml:"local_add"`
}
