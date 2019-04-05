package slurm_operator

//go:generate go run vendor/k8s.io/code-generator/cmd/deepcopy-gen/main.go -O zz_generated.deepcopy -i ./pkg/operator/apis/slurm/v1alpha1 -h COPYRIGHT
//go:generate go run vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -o pkg/operator/apis/slurm -O zz_generated.openapi -p v1alpha1 -i ./pkg/operator/apis/slurm/v1alpha1 -h COPYRIGHT
//go:generate go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd --output-dir deploy/crds --apis-path pkg/operator/apis
