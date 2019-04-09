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
	"path"
	"syscall"

	"github.com/pkg/errors"

	"github.com/fsnotify/fsnotify"

	rd "github.com/sylabs/slurm-operator/internal/resource-daemon"
	"github.com/sylabs/slurm-operator/internal/resource-daemon/k8s"
	"gopkg.in/yaml.v2"
)

const (
	envMyNodeName = "MY_NODE_NAME"
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
	slurmConfigMapPath := flag.String("slurm-config-map", "", "path to attached config map volume with slurm config")
	nodeConfigPath := flag.String("node-config", "", "slurm config path on host machine")
	flag.Parse()

	if *slurmConfigMapPath == "" {
		log.Fatal("slurm config-map path cannot be empty")
	}

	if *nodeConfigPath == "" {
		log.Fatal("node config path cannot be empty")
	}

	if err := patchNode(*slurmConfigMapPath, *nodeConfigPath); err != nil {
		log.Fatal(err)
	}

	sCh := make(chan os.Signal, 1)
	signal.Notify(sCh, syscall.SIGINT, syscall.SIGTERM)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	dirToListen := path.Dir(*slurmConfigMapPath)

	log.Printf("Start listening %s for changes\n", dirToListen)
	if err := watcher.Add(dirToListen); err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case s := <-sCh:
			log.Printf("Finished with: %s", s)
		case err := <-watcher.Errors:
			log.Printf("Watcher err: %s\n", err)
		case e := <-watcher.Events:
			if e.Op&fsnotify.Remove == fsnotify.Remove {
				if err := patchNode(*slurmConfigMapPath, *nodeConfigPath); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func patchNode(cfgP, targetCfgP string) error {
	config, err := readConfig(cfgP)
	if err == errNotConfigured {
		log.Printf("No node configuration was found, skipping configuration\n")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "could not configure resource daemon")
	}

	if err = writeRemoteClusterConfig(targetCfgP, config.SlurmSSHAddress, config.SlurmLocalAddress); err != nil {
		return errors.Wrap(err, "could not write remote cluster config")
	}

	client, err := k8s.NewClient()
	if err != nil {
		return errors.Wrap(err, "could not create k8s client")
	}

	if err = configureLabels(client, config); err != nil {
		return errors.Wrap(err, "could not configure node labels")
	}
	if err = configureResources(client, config); err != nil {
		return errors.Wrap(err, "could not configure node resources")
	}

	return nil
}

func readConfig(p string) (*config, error) {
	nodeName := os.Getenv(envMyNodeName)
	if nodeName == "" {
		return nil, errors.Errorf("missing %s environment variable", envMyNodeName)
	}
	log.Printf("Coufigured to work as %s node", nodeName)

	f, err := os.Open(p)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open config at: %s", p)
	}
	defer func() {
		f.Close()
	}()

	var cfg map[string]config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal config")
	}

	nodeConfig, ok := cfg[nodeName]
	if !ok {
		return nil, errNotConfigured
	}
	if nodeConfig.SlurmSSHAddress == "" && nodeConfig.SlurmLocalAddress == "" {
		return nil, errors.New("either ssh or local SLURM address have to be specified in config map")
	}

	nodeConfig.NodeName = nodeName
	return &nodeConfig, nil
}

func writeRemoteClusterConfig(path, slurmSSHAddr, slurmLocalAddr string) error {
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "could not create slurm config file")
	}
	defer f.Close()

	var nodeConfig = rd.NodeConfig{
		SSHAddr:   slurmSSHAddr,
		LocalAddr: slurmLocalAddr,
	}
	if err = yaml.NewEncoder(f).Encode(nodeConfig); err != nil {
		return errors.Wrap(err, "could not encode node config")
	}
	return nil
}

func configureLabels(k8sClient *k8s.Client, config *config) error {
	if err := k8sClient.AddNodeLabels(config.NodeName, defaultNodeLabels); err != nil {
		return errors.Wrap(err, "could not label node")
	}
	log.Printf("Added default node labels: %v", defaultNodeLabels)

	if len(config.NodeLabels) == 0 {
		log.Println("Custom labels are empty, skipping")
		return nil
	}

	if err := k8sClient.AddNodeLabels(config.NodeName, config.NodeLabels); err != nil {
		return errors.Wrap(err, "could not label node")
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
		return errors.Wrap(err, "could not add node resources")
	}
	log.Printf("Added custom node resources: %v", config.NodeResources)

	return nil
}
