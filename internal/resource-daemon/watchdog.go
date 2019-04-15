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

//nolint:golint
package resource_daemon

import (
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/internal/k8s"
	"gopkg.in/yaml.v2"
)

// NodeConfig contains SLURM cluster local address.
// NodeConfig is written into a config file created by resource-daemon creates on each k8s node.
// Job-companion uses addresses from the file for communicating with SLURM cluster.
type NodeConfig struct {
	Addr string `yaml:"addr"`
}

type Patch struct {
	RedBoxAddress string            `yaml:"red_box_addr"`
	NodeLabels    map[string]string `yaml:"labels"`
	NodeResources map[string]int    `yaml:"resources"`
}

type WatchDog struct {
	NodeName       string
	NodeConfigPath string
	Client         *k8s.Client
	DefaultLabels  map[string]string

	latestPatch *Patch
}

// PatchNode applies passed patch to the node WatchDog is configured to work
// with. PatchNode cleans up any patch applied before.
func (w *WatchDog) PatchNode(patch *Patch) error {
	if err := w.CleanNode(); err != nil {
		return errors.Wrap(err, "could not cleanup node before patching")
	}
	if err := writeNodeConfig(w.NodeConfigPath, patch.RedBoxAddress); err != nil {
		return errors.Wrap(err, "could not write remote cluster config")
	}
	if err := w.configureLabels(patch.NodeLabels); err != nil {
		return errors.Wrap(err, "could not configure node labels")
	}
	if err := w.configureResources(patch.NodeResources); err != nil {
		return errors.Wrap(err, "could not configure node resources")
	}
	return nil
}

// CleanNode reverts changes introduced with the latest PatchNode call.
func (w *WatchDog) CleanNode() error {
	err := os.Remove(w.NodeConfigPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("Could not delete config on the node: %s", err)
	}

	if w.latestPatch == nil {
		return nil
	}

	if len(w.latestPatch.NodeLabels) != 0 {
		log.Printf("Cleaning up node labels")
		err := w.Client.RemoveNodeLabels(w.NodeName, w.latestPatch.NodeLabels)
		if err != nil {
			log.Printf("Could not remove node labels: %s", err)
		}
		err = w.Client.RemoveNodeLabels(w.NodeName, w.DefaultLabels)
		if err != nil {
			log.Printf("Could not remove node labels: %s", err)
		}
	}

	if len(w.latestPatch.NodeResources) != 0 {
		log.Printf("Cleaning up node resources")
		err := w.Client.RemoveNodeResources(w.NodeName, w.latestPatch.NodeResources)
		if err != nil {
			log.Printf("Could not remove node resources: %s", err)
		}
	}
	return nil
}

func (w *WatchDog) configureLabels(labels map[string]string) error {
	if err := w.Client.AddNodeLabels(w.NodeName, w.DefaultLabels); err != nil {
		return errors.Wrap(err, "could not label node")
	}
	log.Printf("Added default node labels: %v", w.DefaultLabels)

	if len(labels) == 0 {
		log.Println("Custom labels are empty, skipping")
		return nil
	}

	if err := w.Client.AddNodeLabels(w.NodeName, labels); err != nil {
		return errors.Wrap(err, "could not label node")
	}
	log.Printf("Added custom node labels: %v", labels)

	return nil
}

func (w *WatchDog) configureResources(resources map[string]int) error {
	if len(resources) == 0 {
		log.Println("Custom resources are empty, skipping")
		return nil
	}

	if err := w.Client.AddNodeResources(w.NodeName, resources); err != nil {
		return errors.Wrap(err, "could not add node resources")
	}
	log.Printf("Added custom node resources: %v", resources)

	return nil
}

func writeNodeConfig(path, redBoxAddr string) error {
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "could not create slurm config file")
	}
	defer f.Close()

	var nodeConfig = NodeConfig{
		Addr: redBoxAddr,
	}
	if err = yaml.NewEncoder(f).Encode(nodeConfig); err != nil {
		return errors.Wrap(err, "could not encode node config")
	}
	return nil
}
