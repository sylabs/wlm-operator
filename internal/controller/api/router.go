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

package api

import (
	"github.com/gorilla/mux"
	"github.com/sylabs/slurm-operator/pkg/slurm"
)

type api struct {
	slurm slurm.Slurm
}

// NewSlurmRouter creates new HTTP router that can be used for
// serving Slurm HTTP requests.
func NewSlurmRouter(sClient slurm.Slurm) *mux.Router {
	a := &api{slurm: sClient}

	r := mux.NewRouter()
	r.HandleFunc("/srun", a.SRun).Methods("POST")
	r.HandleFunc("/sbatch", a.SBatch).Methods("POST")
	r.HandleFunc("/sacct/{id}", a.SAcct).Methods("GET")
	r.HandleFunc("/scancel/{id}", a.SCancel).Methods("GET")
	r.HandleFunc("/open", a.Open).Methods("GET")

	return r
}
