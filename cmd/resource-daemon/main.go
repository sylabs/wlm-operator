// Copyright (c) 2019 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	rd "github.com/sylabs/slurm-operator/internal/resource-daemon"
	"github.com/sylabs/slurm-operator/internal/resource-daemon/k8s"
	"gopkg.in/yaml.v2"
)

const (
	envMyNodeName         = "MY_NODE_NAME"
	envSlurmClusterConfig = "SLURM_CLUSTER_CONFIG"
)

type config struct {
	SlurmSSHAddress   string `yaml:"slurm_ssh"`
	SlurmLocalAddress string `yaml:"slurm_local"`
	NodeName          string
	NodeLabels        map[string]string `yaml:"labels"`
	NodeResources     map[string]int    `yaml:"resources"`
}

var defaultNodeLabels = map[string]string{
	"workload-manager": "slurm",
}

var errNotConfigured = fmt.Errorf("node is not configured")

func main() {
	configPath := flag.String("config", "", "slurm config path on host machine")
	flag.Parse()

	if *configPath == "" {
		log.Fatalf("config path cannot be empty")
	}

	sCh := make(chan os.Signal, 1)
	signal.Notify(sCh, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		sig := <-sCh
		log.Printf("Finished with %s\n", sig.String())
	}()

	config, err := readConfig()
	if err == errNotConfigured {
		log.Printf("No node configuration was found, skipping configuration")
		return
	}
	if err != nil {
		log.Fatalf("could not configure resource daemon: %v", err)
	}

	if err = writeRemoteClusterConfig(*configPath, config.SlurmSSHAddress, config.SlurmLocalAddress); err != nil {
		log.Fatalf("could not write remote cluster config: %v", err)
	}

	client, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("could not create k8s client: %v", err)
	}

	if err = configureLabels(client, config); err != nil {
		log.Fatalf("could not configure node labels: %v", err)
	}
	if err = configureResources(client, config); err != nil {
		log.Fatalf("could not configure node resources: %v", err)
	}
}

func readConfig() (*config, error) {
	nodeName := os.Getenv(envMyNodeName)
	if nodeName == "" {
		return nil, fmt.Errorf("missing %s environment variable", envMyNodeName)
	}
	log.Printf("Coufigured to work as %s node", nodeName)

	clusterMappingStr := os.Getenv(envSlurmClusterConfig)
	if clusterMappingStr == "" {
		return nil, fmt.Errorf("missing %s environment variable", envSlurmClusterConfig)
	}

	var cfg map[string]config
	if err := yaml.Unmarshal([]byte(clusterMappingStr), &cfg); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %v", err)
	}

	nodeConfig, ok := cfg[nodeName]
	if !ok {
		return nil, errNotConfigured
	}
	if nodeConfig.SlurmSSHAddress == "" && nodeConfig.SlurmLocalAddress == "" {
		return nil, fmt.Errorf("whether ssh or local SLURM address have to be specified in config map")
	}

	nodeConfig.NodeName = nodeName
	return &nodeConfig, nil
}

func writeRemoteClusterConfig(path, slurmSSHAddr, slurmLocalAddr string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create slurm config file: %v", err)
	}
	defer f.Close()

	var nodeConfig = rd.NodeConfig{
		SSHAddr:   slurmSSHAddr,
		LocalAddr: slurmLocalAddr,
	}
	if err = yaml.NewEncoder(f).Encode(nodeConfig); err != nil {
		return fmt.Errorf("could not encode node config: %v", err)
	}
	return nil
}

func configureLabels(k8sClient *k8s.Client, config *config) error {
	if err := k8sClient.AddNodeLabels(config.NodeName, defaultNodeLabels); err != nil {
		return fmt.Errorf("could not label node: %v", err)
	}
	log.Printf("Added default node labels: %v", defaultNodeLabels)

	if len(config.NodeLabels) == 0 {
		log.Println("Custom labels are empty, skipping")
		return nil
	}

	if err := k8sClient.AddNodeLabels(config.NodeName, config.NodeLabels); err != nil {
		return fmt.Errorf("could not label node: %v", err)
	}
	log.Printf("Added custom node labels: %v", config.NodeLabels)

	return nil
}

func configureResources(k8sClient *k8s.Client, config *config) error {
	if len(config.NodeResources) == 0 {
		log.Println("Custom resources are empty, skipping")
		return nil
	}

	if err := k8sClient.AddNodeResources(config.NodeName, config.NodeResources); err != nil {
		return fmt.Errorf("could not add node resources: %v", err)
	}
	log.Printf("Added custom node resources: %v", config.NodeResources)

	return nil
}
