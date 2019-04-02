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
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const (
	opAdd = "add"
)

type operation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// Client provides convenient API for interacting with k8s core API.
type Client struct {
	coreClient *corev1.CoreV1Client
}

// NewClient fetches k8s config and initializes core client based on it.
func NewClient() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("could not fetch cluster config: %v", err)
	}

	coreClient, err := corev1.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create core client: %v", err)
	}

	return &Client{coreClient: coreClient}, nil
}

// AddNodeResources adds passed resources to node capacity.
func (c *Client) AddNodeResources(nodeName string, resources map[string]int) error {
	// https://kubernetes.io/docs/tasks/administer-cluster/extended-resource-node/
	const k8sResourceT = "/status/capacity/slurm.sylabs.io~1%s"

	ops := make([]operation, 0, len(resources))
	for k, v := range resources {
		op := operation{
			Op:    opAdd,
			Path:  fmt.Sprintf(k8sResourceT, k),
			Value: strconv.Itoa(v),
		}
		ops = append(ops, op)
	}
	var buff bytes.Buffer
	if err := json.NewEncoder(&buff).Encode(ops); err != nil {
		return fmt.Errorf("could not encode resources patch: %v", err)
	}

	_, err := c.coreClient.Nodes().Patch(nodeName, types.JSONPatchType, buff.Bytes(), "status")
	if err != nil {
		return fmt.Errorf("could not patch node resources: %v", err)
	}
	return nil
}

// AddNodeLabels adds passed labels to node labels.
func (c *Client) AddNodeLabels(nodeName string, labels map[string]string) error {
	const k8sLabelT = "/metadata/labels/slurm.sylabs.io~1%s"

	ops := make([]operation, 0, len(labels))
	for k, v := range labels {
		op := operation{
			Op:    opAdd,
			Path:  fmt.Sprintf(k8sLabelT, k),
			Value: v,
		}
		ops = append(ops, op)
	}

	var buff bytes.Buffer
	if err := json.NewEncoder(&buff).Encode(ops); err != nil {
		return fmt.Errorf("could not encode labels patch: %v", err)
	}

	_, err := c.coreClient.Nodes().Patch(nodeName, types.JSONPatchType, buff.Bytes())
	if err != nil {
		return fmt.Errorf("could not patch node labels: %v", err)
	}
	return nil
}
