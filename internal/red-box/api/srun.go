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
	"net/http"
)

// SRunRequest represents submitted SRun request.
type SRunRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// SRun runs passed command with args in Slurm cluster using context.
// If run succeeds its output is returned uninterpreted as a byte slice.
func (a *api) SRun(w http.ResponseWriter, r *http.Request) {
	var sr SRunRequest

	if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if sr.Command == "" {
		http.Error(w, "command must not be empty", http.StatusBadRequest)
		return
	}

	logs, err := a.slurm.SRun(r.Context(), sr.Command, sr.Args...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if logs != nil {
			w.Write(logs)
		}
		w.Write([]byte(err.Error()))
		return
	}

	w.Write(logs)
}
