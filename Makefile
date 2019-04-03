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
	$(V)for dir in `ls -d */ | tr -d /` ; do \
		if [ -a $${dir}/Dockerfile ]; then \
    		echo " PUSH" $${dir} ; \
			(cd  $${dir} && \
			docker build -t sylabsio/slurm:$${dir} . && \
			docker push sylabsio/slurm:$${dir}); \
    	fi ; \
		if [ "$${dir}" = "operator" ]; then \
    		echo " PUSH" $${dir} ; \
			(cd  $${dir} && \
			operator-sdk build sylabsio/slurm:$${dir} && \
			docker push sylabsio/slurm:$${dir}); \
		fi ; \
	done

dep:
	$(V)for dir in `ls -d */ | tr -d /` ; do \
		if [ -a $${dir}/Gopkg.toml ]; then \
	    	echo " DEP" $${dir} ; \
			(cd  $${dir} && dep ensure --vendor-only); \
    	fi ; \
	done
