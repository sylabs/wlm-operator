# silent build
V := @

BIN_DIR := ./bin
RED_BOX := $(BIN_DIR)/red-box

LDFLAGS = -ldflags "-X main.version=`(git describe  --dirty --always 2>/dev/null || echo "unknown") \
          		| sed -e "s/^v//;s/-/_/g;s/_/-/;s/_/./g"`"

all: $(RED_BOX)

$(RED_BOX):
	@echo " GO" $@
	$(V)go build -mod vendor ${LDFLAGS} -o $(RED_BOX) ./cmd/red-box

.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)go clean -mod vendor
	$(V)rm -rf $(BIN_DIR)

.PHONY: test
test:
	$(V)go test -mod vendor -v -coverprofile=cover.out -race ./...

.PHONY: lint
lint:
	$(V)golangci-lint run --config .golangci.local.yml

.PHONY: push
push: TAG=latest
push:
	$(V)for f in `ls sif` ; do \
		echo " PUSH" $${f}:${TAG} ; \
		sudo singularity build sif/$${f}.sif sif/$${f} ;\
		singularity sign sif/$${f}.sif;\
		singularity push sif/$${f}.sif library://library/slurm/$${f}:${TAG};\
	done

.PHONY: dep
dep:
	$(V)go mod tidy
	$(V)go mod vendor

.PHONY: gen
gen:
	$(V)go generate generate.go

.PHONY: release_binaries
release_binaries:
	$(V)go get -u github.com/itchio/gothub
	$(V)gothub upload --user sylabs \
	--repo wlm-operator \
	--tag ${RELEASE_TAG} \
	--name "red-box" \
	--file ${BINARY_PATH}

