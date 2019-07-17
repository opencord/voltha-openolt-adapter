#
# Copyright 2016 the original author or authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# set default shell
SHELL = bash -e -o pipefail

# Variables
VERSION                  ?= $(shell cat ./VERSION)

DOCKER_LABEL_VCS_DIRTY     = false
ifneq ($(shell git ls-files --others --modified --exclude-standard 2>/dev/null | wc -l | sed -e 's/ //g'),0)
    DOCKER_LABEL_VCS_DIRTY = true
endif
## Docker related
DOCKER_EXTRA_ARGS        ?=
DOCKER_REGISTRY          ?=
DOCKER_REPOSITORY        ?=
DOCKER_TAG               ?= ${VERSION}
GOADAPTER_IMAGENAME      := ${DOCKER_REGISTRY}${DOCKER_REPOSITORY}voltha-openolt-adapter:${DOCKER_TAG}-go
PYTHONADAPTER_IMAGENAME  := ${DOCKER_REGISTRY}${DOCKER_REPOSITORY}voltha-openolt-adapter:${DOCKER_TAG}-py
ADAPTER_IMAGENAME        := ${DOCKER_REGISTRY}${DOCKER_REPOSITORY}voltha-openolt-adapter:${DOCKER_TAG}

## Docker labels. Only set ref and commit date if committed
DOCKER_LABEL_VCS_URL       ?= $(shell git remote get-url $(shell git remote))
DOCKER_LABEL_VCS_REF       = $(shell git rev-parse HEAD)
DOCKER_LABEL_BUILD_DATE    ?= $(shell date -u "+%Y-%m-%dT%H:%M:%SZ")
DOCKER_LABEL_COMMIT_DATE   = $(shell git show -s --format=%cd --date=iso-strict HEAD)

DOCKER_BUILD_ARGS ?= \
	${DOCKER_EXTRA_ARGS} \
	--build-arg org_label_schema_version="${VERSION}" \
	--build-arg org_label_schema_vcs_url="${DOCKER_LABEL_VCS_URL}" \
	--build-arg org_label_schema_vcs_ref="${DOCKER_LABEL_VCS_REF}" \
	--build-arg org_label_schema_build_date="${DOCKER_LABEL_BUILD_DATE}" \
	--build-arg org_opencord_vcs_commit_date="${DOCKER_LABEL_COMMIT_DATE}" \
	--build-arg org_opencord_vcs_dirty="${DOCKER_LABEL_VCS_DIRTY}"

DOCKER_BUILD_ARGS_LOCAL ?= ${DOCKER_BUILD_ARGS} \
	--build-arg LOCAL_PYVOLTHA=${LOCAL_PYVOLTHA} \
	--build-arg LOCAL_PROTOS=${LOCAL_PROTOS}

.PHONY: docker-build openolt_go openolt_python local-protos local-volthago local-pyvoltha

# This should to be the first and default target in this Makefile
help:
	@echo "Usage: make [<target>]"
	@echo "where available targets are:"
	@echo
	@echo "build             : Build both openolt adapter docker images"
	@echo "openolt_go        : Build Golang openolt adapter docker image"
	@echo "openolt_python    : Build Python openolt adapter docker image"
	@echo "help              : Print this help"
	@echo "docker-push       : Push the docker images to an external repository"
	@echo "lint              : Run lint verification, depenancy, gofmt and reference check"
	@echo "sca               : Runs various SCA through golangci-lint tool"
	@echo "test              : Run unit tests, if any"
	@echo


## Local Development Helpers

local-protos:
	mkdir -p python/local_imports
ifdef LOCAL_PROTOS
	mkdir -p vendor/github.com/opencord/voltha-protos/go
	cp -r ${GOPATH}/src/github.com/opencord/voltha-protos/go/* vendor/github.com/opencord/voltha-protos/go
	mkdir -p python/local_imports/voltha-protos/dist
	cp ../voltha-protos/dist/*.tar.gz python/local_imports/voltha-protos/dist/
endif

local-pyvoltha:
	mkdir -p python/local_imports
ifdef LOCAL_PYVOLTHA
	mkdir -p python/local_imports/pyvoltha/dist
	cp ../pyvoltha/dist/*.tar.gz python/local_imports/pyvoltha/dist/
endif

local-volthago:
ifdef LOCAL_VOLTHAGO
	mkdir -p vendor/github.com/opencord/voltha-go/
	cp -rf ${GOPATH}/src/github.com/opencord/voltha-go/ vendor/github.com/opencord/
	rm -rf vendor/github.com/opencord/voltha-go/vendor
endif


## Python venv dev environment

VENVDIR := python/venv-openolt

venv: distclean local-protos local-pyvoltha
	virtualenv ${VENVDIR};\
        source ./${VENVDIR}/bin/activate ; set -u ;\
	rm ${VENVDIR}/local/bin ${VENVDIR}/local/lib ${VENVDIR}/local/include ;\
        pip install -r python/requirements.txt

ifdef LOCAL_PYVOLTHA
	source ./${VENVDIR}/bin/activate ; set -u ;\
	pip install python/local_imports/pyvoltha/dist/*.tar.gz
endif
ifdef LOCAL_PROTOS
	source ./${VENVDIR}/bin/activate ; set -u ;\
	pip install python/local_imports/voltha-protos/dist/*.tar.gz
endif


## Docker targets

build: docker-build

docker-build: openolt_go openolt_python

openolt_go: local-protos local-volthago
	docker build $(DOCKER_BUILD_ARGS) -t ${GOADAPTER_IMAGENAME} -f docker/Dockerfile.openolt .

openolt_python: local-protos local-pyvoltha
	docker build $(DOCKER_BUILD_ARGS_LOCAL) -t ${PYTHONADAPTER_IMAGENAME} -f python/docker/Dockerfile.openolt_adapter python

	# Current default image gets the base DOCKER_TAG
	docker tag ${PYTHONADAPTER_IMAGENAME} ${ADAPTER_IMAGENAME}

docker-push:
	docker push ${GOADAPTER_IMAGENAME}
	docker push ${PYTHONADAPTER_IMAGENAME}
	docker push ${ADAPTER_IMAGENAME}


## lint and unit tests

lint-style:
ifeq (,$(shell which gofmt))
	go get -u github.com/golang/go/src/cmd/gofmt
endif
	@echo "Running style check..."
	@gofmt_out="$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" ;\
	if [ ! -z "$$gofmt_out" ]; then \
	  echo "$$gofmt_out" ;\
	  echo "Style check failed on one or more files ^, run 'go fmt' to fix." ;\
	  exit 1 ;\
	fi
	@echo "Style check OK"

lint-sanity:
	@echo "Running sanity check..."
	@go vet ./...
	@echo "Sanity check OK"

lint-dep:
	@echo "Running dependency check..."
	@dep check
	@echo "Dependency check OK"

lint: lint-style lint-sanity lint-dep

GO_JUNIT_REPORT:=$(shell which go-junit-report)
GOCOVER_COBERTURA:=$(shell which gocover-cobertura)
test:
ifeq (,$(GO_JUNIT_REPORT))
	go get -u github.com/jstemmer/go-junit-report
	@GO_JUNIT_REPORT=$(GOPATH)/bin/go-junit-report
endif

ifeq (,$(GOCOVER_COBERTURA))
	go get -u github.com/t-yuki/gocover-cobertura
	@GOCOVER_COBERTURA=$(GOPATH)/bin/gocover-cobertura
endif

	@mkdir -p ./tests/results

	@go test -v -coverprofile ./tests/results/go-test-coverage.out -covermode count ./... 2>&1 | tee ./tests/results/go-test-results.out ;\
	RETURN=$$? ;\
	$(GO_JUNIT_REPORT) < ./tests/results/go-test-results.out > ./tests/results/go-test-results.xml ;\
	$(GOCOVER_COBERTURA) < ./tests/results/go-test-coverage.out > ./tests/results/go-test-coverage.xml ;\
	exit $$RETURN

GOLANGCI_LINT_TOOL:=$(shell which golangci-lint)
sca:
ifeq (,$(GOLANGCI_LINT_TOOL))
	@echo "Please install golangci-lint tool to run sca"
	exit 1
endif
	@mkdir -p ./sca-report
	GO111MODULE=on golangci-lint run --out-format junit-xml ./... 2>&1 | tee ./sca-report/sca-report.xml ;\
	RETURN=$$? ;\
	exit $$RETURN

clean:
	rm -rf python/local_imports
	rm -rf sca-report
	find python -name '*.pyc' | xargs rm -f

distclean: clean
	rm -rf ${VENVDIR}

# end file
