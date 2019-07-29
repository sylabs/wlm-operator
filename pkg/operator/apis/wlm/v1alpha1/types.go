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

package v1alpha1

import v1 "k8s.io/api/core/v1"

// JobResults is a schema for results collection.
// +k8s:openapi-gen=true
type JobResults struct {
	// Mount is a directory where job results will be stored.
	// After results collection all job generated files can be found in Mount/<SlurmJob.Name> directory.
	Mount v1.Volume `json:"mount"`

	// From is a path to the results to be collected from a Slurm cluster.
	From string `json:"from"`
}
