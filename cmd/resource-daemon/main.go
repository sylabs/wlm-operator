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
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sylabs/singularity-cri/pkg/fs"
	"github.com/sylabs/slurm-operator/internal/k8s"
	"gopkg.in/yaml.v2"
)

const (
	envMyNodeName    = "MY_NODE_NAME"
	kubeletWatchPath = "/var/lib/kubelet/device-plugins/"
	kubeletSocket    = kubeletWatchPath + "kubelet.sock"
)

var (
	defaultNodeLabels = map[string]string{
		"workload-manager": "slurm",
	}

	errNotConfigured = fmt.Errorf("node is not configured")
)

func main() {
	slurmConfigMapPath := flag.String("slurm-config-map", "", "path to attached config map volume with slurm config")
	flag.Parse()

	if *slurmConfigMapPath == "" {
		log.Fatal("slurm config-map path cannot be empty")
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

	wd := &k8s.WatchDog{
		NodeName:      nodeName,
		Client:        k8sClient,
		DefaultLabels: defaultNodeLabels,
	}

	if err := watchAndUpdate(wd, *slurmConfigMapPath); err != nil {
		log.Fatal(err)
	}
}

func loadPatch(nodeName, path string) (*k8s.Patch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open config at %s", path)
	}
	defer f.Close()

	var cfg map[string]*k8s.Patch
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal config")
	}

	nodeConfig, ok := cfg[nodeName]
	if !ok {
		return nil, errNotConfigured
	}
	return nodeConfig, nil
}

func updateNode(wd *k8s.WatchDog, configPath string) error {
	log.Println("Cleaning node before patching")
	wd.CleanNode()
	config, err := loadPatch(wd.NodeName, configPath)
	if err != nil {
		if err == errNotConfigured {
			log.Println("No node configuration was found, skipping configuration")
			return nil
		}
		return errors.Wrap(err, "could not load patch")
	}

	log.Println("Patching node")
	err = wd.PatchNode(config)
	if err != nil {
		return errors.Wrap(err, "could not patch node")
	}
	return nil
}

func watchAndUpdate(wd *k8s.WatchDog, configPath string) error {
	if err := updateNode(wd, configPath); err != nil {
		return errors.Wrap(err, "could not configure node for the first time")
	}
	defer wd.CleanNode()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	configDir := path.Dir(configPath)
	watcher, err := fs.NewWatcher(configDir, kubeletWatchPath)
	if err != nil {
		return errors.Wrap(err, "could not create file watcher")
	}
	defer watcher.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := watcher.Watch(ctx)

	for {
		select {
		case sig := <-signals:
			log.Printf("Finished due to %s", sig)
			return nil
		case e := <-events:
			// we want to ignore all events except for config removal
			// and kubelet socket creation
			if (filepath.Dir(e.Path) == configDir && e.Op == fs.OpRemove) ||
				(e.Path == kubeletSocket && e.Op == fs.OpCreate) {
				err := updateNode(wd, configPath)
				if err != nil {
					return errors.Wrap(err, "could not update node")
				}
			}
		}
	}
}
