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

	rd "github.com/sylabs/slurm-operator/internal/resource-daemon"

	"github.com/pkg/errors"

	"github.com/fsnotify/fsnotify"

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

var (
	defaultNodeLabels = map[string]string{
		"workload-manager": "slurm",
	}

	errNotConfigured = fmt.Errorf("node is not configured")
)

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

	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("could not create k8s client: %v", err)
	}

	if err := watchAndUpdate(k8sClient, *slurmConfigMapPath, *nodeConfigPath); err != nil {
		log.Fatal(err)
	}
}

func watchAndUpdate(client *k8s.Client, configPath, targetPath string) error {
	var cfg *config
	defer func() {
		cleanUp(client, cfg, targetPath)
	}()

	update := func() error {
		cleanUp(client, cfg, targetPath)

		config, err := loadConfig(configPath)
		if err != nil {
			if err == errNotConfigured {
				log.Println("No node configuration was found, skipping configuration")
				return nil
			}

			return errors.Wrap(err, "could not configure resource daemon")
		}

		if err := patchNode(client, config, targetPath); err != nil {
			return errors.Wrap(err, "could not update node")
		}

		cfg = config

		return nil
	}

	if err := update(); err != nil {
		return err
	}

	sCh := make(chan os.Signal, 1)
	signal.Notify(sCh, syscall.SIGINT, syscall.SIGTERM)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	dirToListen := path.Dir(configPath)

	log.Printf("Start listening %s for changes", dirToListen)
	if err := watcher.Add(dirToListen); err != nil {
		return err
	}

	for {
		select {
		case s := <-sCh:
			log.Printf("Finished with: %s", s)
			return nil
		case err := <-watcher.Errors:
			log.Printf("Watcher err: %s", err)
		case e := <-watcher.Events:
			if e.Op&fsnotify.Remove == fsnotify.Remove {
				if err := update(); err != nil {
					return err
				}
			}
		}
	}
}

func patchNode(client *k8s.Client, cfg *config, targetCfgPath string) error {
	if err := writeRemoteClusterConfig(targetCfgPath, cfg.SlurmSSHAddress, cfg.SlurmLocalAddress); err != nil {
		return errors.Wrap(err, "could not write remote cluster config")
	}

	if err := configureLabels(client, cfg); err != nil {
		return errors.Wrap(err, "could not configure node labels")
	}

	if err := configureResources(client, cfg); err != nil {
		return errors.Wrap(err, "could not configure node resources")
	}

	return nil
}

func loadConfig(p string) (*config, error) {
	nodeName := os.Getenv(envMyNodeName)
	if nodeName == "" {
		return nil, errors.Errorf("missing %s environment variable", envMyNodeName)
	}
	log.Printf("Coufigured to work as %s node", nodeName)

	f, err := os.Open(p)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open config at: %s", p)
	}
	defer f.Close()

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

func cleanUp(k8sClient *k8s.Client, cfg *config, nodeCfgPath string) {
	if err := os.Remove(nodeCfgPath); err != nil {
		log.Printf("Could not delete config on the node: %s", err)
	}

	if cfg == nil {
		log.Println("There are no k8s resources to clean up")
		return
	}

	if err := k8sClient.RemoveNodeLabels(cfg.NodeName, cfg.NodeLabels); err != nil {
		log.Printf("Could not remove node labels: %s", err)
	}

	if err := k8sClient.RemoveNodeResources(cfg.NodeName, cfg.NodeResources); err != nil {
		log.Printf("Could not remove node resources: %s", err)
	}

}
