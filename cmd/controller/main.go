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
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/sylabs/slurm-operator/internal/controller/api"
	"github.com/sylabs/slurm-operator/pkg/slurm/local"
	"golang.org/x/sys/unix"
)

func main() {
	sock := flag.String("socket", "red-box.sock", "unix socket to serve slurm API")
	flag.Parse()

	slurmClient, err := local.NewClient()
	if err != nil {
		log.Fatalf("Could not create new local slurm client: %v", err)
	}

	router := api.NewSlurmRouter(slurmClient)
	ln, err := net.Listen("unix", *sock)
	if err != nil {
		log.Fatalf("Could not listen unix: %v", err)
	}
	defer ln.Close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, unix.SIGINT, unix.SIGTERM, unix.SIGQUIT)

	srv := http.Server{Handler: router}
	go func() {
		log.Printf("Starting server on %s", ln.Addr())
		err := srv.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			log.Printf("Server stopped with error: %v", err)
		}
	}()

	log.Printf("Shutting down due to %v", <-sig)
	err = srv.Shutdown(context.Background())
	if err != nil {
		log.Printf("Could not shutdown server gracefully: %v", err)
	}
}
