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
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sylabs/slurm-operator/pkg/slurm"
)

// SBatchRequest represents submitted SBatch request.
type SBatchRequest struct {
	Command string `json:"command"`
}

// SCancel cancels batch job.
func (a *api) SCancel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idS, ok := vars["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idS, 10, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println(err)
		return
	}

	err = a.slurm.SCancel(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

// SBatch submits batch job and returns job id if succeeded.
func (a *api) SBatch(w http.ResponseWriter, r *http.Request) {
	var sb SBatchRequest

	if err := json.NewDecoder(r.Body).Decode(&sb); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println(err)
		return
	}

	if sb.Command == "" {
		http.Error(w, "command must not be empty", http.StatusBadRequest)
		return
	}

	jid, err := a.slurm.SBatch(sb.Command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(strconv.FormatInt(jid, 10)))
}

// SJobSteps returns information about a submitted batch job.
func (a *api) SJobInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	idS, ok := vars["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idS, 10, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println(err)
		return
	}

	sinfo, err := a.slurm.SJobInfo(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	if err := json.NewEncoder(w).Encode(sinfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

// SJobSteps returns information about steps in a submitted batch job.
func (a *api) SJobSteps(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	idS, ok := vars["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idS, 10, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println(err)
		return
	}

	stepsInfo, err := a.slurm.SJobSteps(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	if err := json.NewEncoder(w).Encode(stepsInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

// Open streams content of an arbitrary file.
func (a *api) Open(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	path := query.Get("path")
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

func (a *api) Tail(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	path := query.Get("path")
	if path == "" {
		http.Error(w, "no path query parameter is found", http.StatusBadRequest)
		return
	}

	file, err := a.slurm.Tail(path)
	if err == slurm.ErrFileNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}

	flusher.Flush()

	throttle := time.Tick(200 * time.Millisecond)

	for {
		select {
		case <-r.Context().Done():
			_ = file.Close()
			log.Println("Client closed connections")
			return
		case <-throttle:
			buff := make([]byte, 128)
			n, err := file.Read(buff)
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return
				}

				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if n == 0 { // just wait till data appear
				continue
			}

			if _, err := w.Write(buff[:n]); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			flusher.Flush()
		}
	}
}
