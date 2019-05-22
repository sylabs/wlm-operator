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

package k8s

import (
	"log"

	"github.com/pkg/errors"
)

// Patch represents changes that need to be applied to the node.
type Patch struct {
	NodeLabels    map[string]string `yaml:"labels"`
	NodeResources map[string]int    `yaml:"resources"`
}

// WatchDog is used to perform node patch and cleanup.
type WatchDog struct {
	NodeName      string
	Client        *Client
	DefaultLabels map[string]string

	latestPatch *Patch
}

// PatchNode applies passed patch to the node WatchDog is configured to work
// with. PatchNode cleans up any patch applied before.
func (w *WatchDog) PatchNode(patch *Patch) error {
	w.latestPatch = patch
	if err := w.configureLabels(patch.NodeLabels); err != nil {
		return errors.Wrap(err, "could not configure node labels")
	}
	if err := w.configureResources(patch.NodeResources); err != nil {
		return errors.Wrap(err, "could not configure node resources")
	}
	return nil
}

// CleanNode reverts changes introduced with the latest PatchNode call.
func (w *WatchDog) CleanNode() {
	if w.latestPatch == nil {
		return
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
}

func (w *WatchDog) configureLabels(l map[string]string) error {
	lbls := make(map[string]string, len(w.DefaultLabels)+len(l))
	for k, v := range w.DefaultLabels {
		lbls[k] = v
	}
	for k, v := range l {
		lbls[k] = v
	}
	if err := w.Client.AddNodeLabels(w.NodeName, lbls); err != nil {
		return errors.Wrap(err, "could not label node")
	}
	log.Printf("Added node labels: %v", lbls)
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
	log.Printf("Added node resources: %v", resources)

	return nil
}
