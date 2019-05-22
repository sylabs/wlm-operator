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
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sylabs/slurm-operator/pkg/workload/api"
	"google.golang.org/grpc"

	"github.com/pkg/errors"
	"github.com/sylabs/singularity-cri/pkg/fs"
	"github.com/sylabs/slurm-operator/internal/k8s"
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
)

func main() {
	sock := flag.String("socket", "/red-box.sock", "unix socket to connect to red-box")
	flag.Parse()

	nodeName := os.Getenv(envMyNodeName)
	if nodeName == "" {
		log.Fatalf("Missing %s environment variable", envMyNodeName)
	}
	log.Printf("Coufigured to work as %s node", nodeName)

	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("could not create k8s client: %v", err)
	}

	conn, err := grpc.Dial("unix://"+*sock, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("can't connect to %s %s", *sock, err)
	}
	client := api.NewWorkloadManagerClient(conn)

	wd := &k8s.WatchDog{
		NodeName:      nodeName,
		Client:        k8sClient,
		DefaultLabels: defaultNodeLabels,
	}

	if err := watchAndUpdate(wd, client); err != nil {
		log.Fatal(err)
	}
}

func updateNode(wd *k8s.WatchDog, wClient api.WorkloadManagerClient) error {
	log.Println("Cleaning node before patching")
	wd.CleanNode()

	resResp, err := wClient.Resources(context.Background(), &api.ResourcesRequest{})
	if err != nil {
		return errors.Wrap(err, "can't get resources from red-box")
	}

	patch := &k8s.Patch{
		NodeLabels: map[string]string{
			"nodes":        strconv.FormatInt(resResp.Nodes, 10),
			"wall-time":    strconv.FormatInt(resResp.WallTime, 10),
			"cpu-per-node": strconv.FormatInt(resResp.CpuPerNode, 10),
			"mem-per-node": strconv.FormatInt(resResp.MemPerNode, 10),
		},
	}

	log.Println("Patching node")
	if err := wd.PatchNode(patch); err != nil {
		return errors.Wrap(err, "could not patch node")
	}
	return nil
}

func watchAndUpdate(wd *k8s.WatchDog, wClient api.WorkloadManagerClient) error {
	if err := updateNode(wd, wClient); err != nil {
		return errors.Wrap(err, "could not configure node for the first time")
	}
	defer wd.CleanNode()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	watcher, err := fs.NewWatcher(kubeletWatchPath)
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
			if e.Path == kubeletSocket && e.Op == fs.OpCreate {
				if err := updateNode(wd, wClient); err != nil {
					return errors.Wrap(err, "could not update node")
				}
			}
		case <-time.NewTicker(15 * time.Second).C:
			if err := updateNode(wd, wClient); err != nil {
				return errors.Wrap(err, "could not update node")
			}
		}
	}
}
