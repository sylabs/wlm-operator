# silent build
V := @

BIN_DIR := ./bin
SINGULARITY_RD := $(BIN_DIR)/singularity-rd

all: $(SINGULARITY_RD)

$(SINGULARITY_RD):
	@echo " GO" $@
	$(V)go build -o $(SINGULARITY_RD) ./cmd

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

dep:
	$(V)dep ensure --vendor-only
