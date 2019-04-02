# slurm-controller

The part which communicates with SLURM.



## Installation 

Build as a simple golang app `go build cmd/server/server.go` && `./server -port=8080` start's http server at localhost:8080. It's important to start it on the same machine with slurm master.