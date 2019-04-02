# silent build
V := @

BIN_DIR := ./bin
SLURM_CONTROLLER := $(BIN_DIR)/slurm-controller

all: $(SLURM_CONTROLLER)

$(SLURM_CONTROLLER):
	@echo " GO" $@
	$(V)go build -o $(SLURM_CONTROLLER) ./controller/cmd

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
	$(V)cd job-companion
	$(V)docker build -t sylabsio/slurm:job-companion .
	$(V)docker push sylabsio/slurm:job-companion

dep:
	$(V)for dir in `ls -d */` ; do \
    	echo " DEP" $${dir} ; \
    	(cd  $${dir} && dep ensure --vendor-only); \
	done
