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

//nolint:golint
package resource_daemon

// NodeConfig contains SLURM cluster local address.
// NodeConfig is written into a config file created by resource-daemon creates on each k8s node.
// Job-companion uses addresses from the file for communicating with SLURM cluster.
type NodeConfig struct {
	Addr string `yaml:"addr"`
}
