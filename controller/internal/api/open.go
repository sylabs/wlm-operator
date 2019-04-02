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
	"io"
	"log"
	"net/http"
	"os"

	"github.com/sylabs/slurm-operator/controller/pkg/slurm"
)

// Open streams content of an arbitrary file.
func (a *api) Open(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	d, _ := os.Getwd()
	log.Println(d)
	path := query.Get("path")
	log.Println(path)
	if path == "" {
		http.Error(w, "no path query parameter is found", http.StatusBadRequest)
		return
	}

	file, err := a.slurm.Open(path)
	if err == slurm.ErrFileNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
