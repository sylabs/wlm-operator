# silent build
V := @

BIN_DIR := ./bin
RED_BOX := $(BIN_DIR)/red-box
CONFIGURATOR_CMD := $(BIN_DIR)/configurator
OPERATOR_CMD := $(BIN_DIR)/operator
RESULTS_CMD := $(BIN_DIR)/results

all: $(RED_BOX) $(CONFIGURATOR_CMD) $(OPERATOR_CMD) $(RESULTS_CMD)

$(RED_BOX):
	@echo " GO" $@
	$(V)go build -o $(RED_BOX) ./cmd/red-box

$(CONFIGURATOR_CMD):
	@echo " GO" $@
	$(V)go build -o $(CONFIGURATOR_CMD) ./cmd/configurator

$(OPERATOR_CMD):
	@echo " GO" $@
	$(V)go build -o $(OPERATOR_CMD) ./cmd/operator

$(RESULTS_CMD):
	@echo " GO" $@
	$(V)go build -o $(RESULTS_CMD) ./cmd/results

.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)go clean
	$(V)rm -rf $(BIN_DIR)

.PHONY: test
test:
	$(V)go test -v -coverprofile=cover.out -race ./...

.PHONY: lint
lint:
	$(V)golangci-lint run

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
	$(V)dep ensure --vendor-only

.PHONY: gen
gen:
	$(V)go generate generate.go

.PHONY: release_binaries
release_binaries:
	$(V)go get -u github.com/itchio/gothub
	$(V)gothub upload --user sylabs \
	--repo slurm-operator \
	--tag ${RELEASE_TAG} \
	--name "red-box" \
	--file ${BINARY_PATH}

