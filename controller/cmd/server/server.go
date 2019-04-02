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
	"net/http"

	"github.com/sylabs/slurm-operator/controller/internal/api"
	"github.com/sylabs/slurm-operator/controller/pkg/slurm/local"
)

func main() {
	port := flag.Int("port", 8080, "port at which server will run")

	flag.Parse()

	slurmClient, err := local.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	router := api.NewSlurmRouter(slurmClient)
	log.Printf("server started at %d", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), router); err != nil {
		log.Fatal(err)
	}
}
