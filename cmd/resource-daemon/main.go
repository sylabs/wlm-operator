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

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/internal/k8s"
	rd "github.com/sylabs/slurm-operator/internal/resource-daemon"
	"gopkg.in/yaml.v2"
)

const (
	envMyNodeName = "MY_NODE_NAME"
)

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

	nodeName := os.Getenv(envMyNodeName)
	if nodeName == "" {
		log.Fatalf("Missing %s environment variable", envMyNodeName)
	}
	log.Printf("Coufigured to work as %s node", nodeName)

	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("could not create k8s client: %v", err)
	}

	wd := &rd.WatchDog{
		NodeName:       nodeName,
		NodeConfigPath: *nodeConfigPath,
		Client:         k8sClient,
		DefaultLabels:  defaultNodeLabels,
	}

	if err := watchAndUpdate(wd, *slurmConfigMapPath); err != nil {
		log.Fatal(err)
	}
}

func loadPatch(nodeName, path string) (*rd.Patch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open config at %s", path)
	}
	defer f.Close()

	var cfg map[string]*rd.Patch
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal config")
	}

	nodeConfig, ok := cfg[nodeName]
	if !ok {
		return nil, errNotConfigured
	}
	if nodeConfig.RedBoxAddress == "" {
		return nil, errors.New("SLURM local address have to be specified in config map")
	}
	return nodeConfig, nil
}

func updateNode(wd *rd.WatchDog, configPath string) error {
	config, err := loadPatch(wd.NodeName, configPath)
	if err != nil {
		if err == errNotConfigured {
			log.Println("No node configuration was found, skipping configuration")
			return nil
		}
		return errors.Wrap(err, "could not load patch")
	}

	err = wd.PatchNode(config)
	if err != nil {
		return errors.Wrap(err, "could not patch node")
	}
	return nil
}

func watchAndUpdate(wd *rd.WatchDog, configPath string) error {
	if err := updateNode(wd, configPath); err != nil {
		return errors.Wrap(err, "could not configure node for the first time")
	}
	defer wd.CleanNode()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrap(err, "could not create file watcher")
	}
	defer watcher.Close()

	dirToListen := path.Dir(configPath)
	log.Printf("Start listening %s for changes", dirToListen)
	if err := watcher.Add(dirToListen); err != nil {
		return errors.Wrapf(err, "could not subscribe to %s changes", dirToListen)
	}

	for {
		select {
		case sig := <-signals:
			log.Printf("Finished due to %s", sig)
			return nil
		case err := <-watcher.Errors:
			log.Printf("Watcher err: %s", err)
		case e := <-watcher.Events:
			if e.Op&fsnotify.Remove != fsnotify.Remove {
				continue
			}
			err := updateNode(wd, configPath)
			if err != nil {
				return errors.Wrap(err, "could not update node")
			}
		}
	}
}
