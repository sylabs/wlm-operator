// nolint: golint
package resource_daemon

// NodeConfig contains SLURM cluster local address.
// NodeConfig is written into a config file created by resource-daemon creates on each k8s node.
// Job-companion uses addresses from the file for communicating with SLURM cluster.
type NodeConfig struct {
	Addr string `yaml:"addr"`
}
