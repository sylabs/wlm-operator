# silent build
V := @

BIN_DIR := ./bin
RED_BOX := $(BIN_DIR)/red-box

all: $(RED_BOX)

$(RED_BOX):
	@echo " GO" $@
	$(V)go build -o $(RED_BOX) ./cmd/red-box

.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)go clean
	$(V)rm -rf $(BIN_DIR)

.PHONY: test
test:
	$(V)go test -v ./...

.PHONY: lint
lint:
	$(V)golangci-lint run --disable-all \
	--enable=gofmt \
	--enable=goimports \
	--enable=vet \
	--enable=misspell \
	--enable=maligned \
	--enable=deadcode \
	--enable=ineffassign \
	--enable=golint \
	--deadline=3m ./...

.PHONY: push
push:
	$(V)for f in `ls sif` ; do \
		echo " PUSH" $${f} ; \
		sudo singularity build sif/$${f}.sif sif/$${f} ;\
		singularity sign sif/$${f}.sif;\
		singularity push sif/$${f}.sif library://library/slurm/$${f}:dev-latest;\
	done

.PHONY: dep
dep:
	$(V)dep ensure --vendor-only

.PHONY: gen
gen:
	$(V)go generate generate.go
