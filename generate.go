package slurm_operator

//go:generate controller-gen crd --output-dir deploy/crds --apis-path pkg/operator/apis
//go:generate openapi-gen -o pkg/operator/apis/slurm -O zz_generated.openapi -p v1alpha1 -i ./pkg/operator/apis/slurm/v1alpha1 -h COPYRIGHT
//go:generate deepcopy-gen -O zz_generated.deepcopy -i ./pkg/operator/apis/slurm/v1alpha1 -h COPYRIGHT
