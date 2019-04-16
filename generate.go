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
package slurm_operator

//go:generate go run vendor/k8s.io/code-generator/cmd/deepcopy-gen/main.go -O zz_generated.deepcopy -i ./pkg/operator/apis/slurm/v1alpha1 -h COPYRIGHT
//go:generate go run vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -o pkg/operator/apis/slurm -O zz_generated.openapi -p v1alpha1 -i ./pkg/operator/apis/slurm/v1alpha1 -h COPYRIGHT
//go:generate go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd --output-dir deploy/crds --apis-path pkg/operator/apis
