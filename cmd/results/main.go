package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/sylabs/slurm-operator/pkg/workload/api"
	"google.golang.org/grpc"
)

var (
	from = flag.String("from", "", "specify file name to collect")
	to   = flag.String("to", "", "specify directory where to put file")

	redBoxSock = flag.String("sock", "", "path to red-box socket")
)

func main() {
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
