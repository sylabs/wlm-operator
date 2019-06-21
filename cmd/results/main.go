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
	"io"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/sylabs/wlm-operator/pkg/workload/api"
	"google.golang.org/grpc"
)

var (
	version = "unknown"

	from = flag.String("from", "", "specify file name to collect")
	to   = flag.String("to", "", "specify directory where to put file")

	redBoxSock = flag.String("sock", "", "path to red-box socket")
)

func main() {
	fmt.Printf("version: %s\n", version)

	flag.Parse()

	if *from == "" {
		panic("from can't be empty")
	}

	if *to == "" {
		panic("to can't be empty")
	}

	if *redBoxSock == "" {
		panic("path to red-box socket can't be empty")
	}

	conn, err := grpc.Dial("unix://"+*redBoxSock, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("can't connect to %s %s", *redBoxSock, err)
	}
	client := api.NewWorkloadManagerClient(conn)

	openReq, err := client.OpenFile(context.Background(), &api.OpenFileRequest{Path: *from})
	if err != nil {
		log.Fatalf("can't open file err: %s", err)
	}

	if err := os.MkdirAll(*to, 0755); err != nil {
		log.Fatalf("can't create dir on mounted volume err: %s", err)
	}

	filePath := path.Join(*to, filepath.Base(*from))
	toFile, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("can't not create file with results on mounted volume err: %s", err)
	}
	defer toFile.Close()

	for {
		chunk, err := openReq.Recv()

		if chunk != nil {
			if _, err := toFile.Write(chunk.Content); err != nil {
				log.Fatalf("can't write to file err: %s", err)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatalf("err while receiving file %s", err)
		}
	}

	log.Println("Collecting results ended")
	log.Printf("File is located at %s", filePath)
}
