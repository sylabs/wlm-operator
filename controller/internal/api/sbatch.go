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
	"log"
	"net/http"
	"strconv"
)

// SBatchRequest represents submitted SBatch request.
type SBatchRequest struct {
	Command string `json:"command"`
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
