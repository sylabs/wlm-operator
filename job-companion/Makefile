# silent build
V := @

BIN_DIR := ./bin
JOB_COMPANION := $(BIN_DIR)/job-companion

all: $(JOB_COMPANION)

$(JOB_COMPANION):
	@echo " GO" $@
	$(V)GOOS=linux CGO_ENABLED=0 go build -o $(JOB_COMPANION) ./cmd

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
	$(V)docker build -t sylabsio/slurm:job-companion .
	$(V)docker push sylabsio/slurm:job-companion

dep:
	$(V)dep ensure --vendor-only
