package controller

import (
	"github.com/sylabs/slurm-operator/operator/pkg/controller/slurmjob"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, slurmjob.Add)
}
