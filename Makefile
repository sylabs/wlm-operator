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
	$(V)for f in `ls docker | sed  's/Dockerfile.//'` ; do \
		echo " PUSH" $${f} ; \
		docker build -f docker/Dockerfile.$${f} -t sylabsio/slurm:$${f} . ;\
		docker push sylabsio/slurm:$${f} ;\
	done

.PHONY: dep
dep:
	$(V)dep ensure --vendor-only

.PHONY: gen
gen:
	$(V)go generate generate.go
