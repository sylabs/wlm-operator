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
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/sylabs/slurm-operator/pkg/slurm"

	sgrpc "github.com/sylabs/slurm-operator/internal/red-box/api"
	"github.com/sylabs/slurm-operator/pkg/workload/api"

	"golang.org/x/sys/unix"

	"google.golang.org/grpc"
)

func main() {
	sock := flag.String("socket", "/var/run/syslurm/red-box.sock", "unix socket to serve slurm API")
	flag.Parse()

	ln, err := net.Listen("unix", *sock)
	if err != nil {
		log.Fatalf("Could not listen unix: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	s := grpc.NewServer()

	c, err := slurm.NewClient()
	if err != nil {
		log.Fatalf("Could not create slurm client: %s", err)
	}

	a := sgrpc.NewSlurmAPI(c)
	api.RegisterWorkloadManagerServer(s, a)

	go func() {
		defer wg.Done()

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, unix.SIGINT, unix.SIGTERM, unix.SIGQUIT)
		log.Printf("Shutting down due to %v", <-sig)
		s.GracefulStop()
	}()

	log.Printf("Starting server on %s", ln.Addr())
	if err := s.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not serve requests: %v", err)
	}
	wg.Wait()
}
