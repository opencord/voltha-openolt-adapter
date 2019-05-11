#
# Copyright 2018 the original author or authors.
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
VERSION                  ?= $(shell cat ../VERSION)
ADAPTER_NAME             ?= voltha-openolt-adapter

## Docker related
DOCKER_REGISTRY          ?=
DOCKER_REPOSITORY        ?=
DOCKER_BUILD_ARGS        ?=
DOCKER_TAG               ?= ${VERSION}
ADAPTER_IMAGENAME        := ${DOCKER_REGISTRY}${DOCKER_REPOSITORY}${ADAPTER_NAME}:${DOCKER_TAG}

## Docker labels. Only set ref and commit date if committed
DOCKER_LABEL_VCS_URL     ?= $(shell git remote get-url $(shell git remote))
DOCKER_LABEL_VCS_REF     ?= $(shell git diff-index --quiet HEAD -- && git rev-parse HEAD || echo "unknown")
DOCKER_LABEL_COMMIT_DATE ?= $(shell git diff-index --quiet HEAD -- && git show -s --format=%cd --date=iso-strict HEAD || echo "unknown" )
DOCKER_LABEL_BUILD_DATE  ?= $(shell date -u "+%Y-%m-%dT%H:%M:%SZ")

.PHONY: venv docker-build local-protos local-pyvoltha

# This should to be the first and default target in this Makefile
help:
	@echo "Usage: make [<target>]"
	@echo "where available targets are:"
	@echo
	@echo "build             : Build the openolt adapter docker image"
	@echo "help              : Print this help"
	@echo "docker-push       : Push the docker images to an external repository"
	@echo "venv              : Build local Python virtualenv"
	@echo "clean             : Remove files created by the build and tests"
	@echo "distclean         : Remove venv directory"
	@echo


## Docker targets

build: docker-build

local-protos: 
	mkdir -p local_imports
ifdef LOCAL_PROTOS
	mkdir -p local_imports/voltha-protos/dist
	cp ../../voltha-protos/dist/*.tar.gz local_imports/voltha-protos/dist/
endif

local-pyvoltha:
	mkdir -p local_imports
ifdef LOCAL_PYVOLTHA
	mkdir -p local_imports/pyvoltha/dist
	cp ../../pyvoltha/dist/*.tar.gz local_imports/pyvoltha/dist/
endif

docker-build: local-protos local-pyvoltha
	docker build $(DOCKER_BUILD_ARGS) \
    -t ${ADAPTER_IMAGENAME} \
    --build-arg LOCAL_PYVOLTHA=$(LOCAL_PYVOLTHA) \
    --build-arg LOCAL_PROTOS=$(LOCAL_PROTOS) \
    --build-arg org_label_schema_name="${ADAPTER_NAME}" \
    --build-arg org_label_schema_version="${VERSION}" \
    --build-arg org_label_schema_vcs_url="${DOCKER_LABEL_VCS_URL}" \
    --build-arg org_label_schema_vcs_ref="${DOCKER_LABEL_VCS_REF}" \
    --build-arg org_label_schema_build_date="${DOCKER_LABEL_BUILD_DATE}" \
    --build-arg org_opencord_vcs_commit_date="${DOCKER_LABEL_COMMIT_DATE}" \
    -f docker/Dockerfile.openolt_adapter .

docker-push:
	docker push ${ADAPTER_IMAGENAME}


## Python venv dev environment

VENVDIR := venv-openolt

venv: distclean local-protos local-pyvoltha
	virtualenv ${VENVDIR};\
        source ./${VENVDIR}/bin/activate ; set -u ;\
	rm ${VENVDIR}/local/bin ${VENVDIR}/local/lib ${VENVDIR}/local/include ;\
        pip install -r requirements.txt

ifdef LOCAL_PYVOLTHA
	source ./${VENVDIR}/bin/activate ; set -u ;\
	pip install local_imports/pyvoltha/dist/*.tar.gz
endif
ifdef LOCAL_PROTOS
	source ./${VENVDIR}/bin/activate ; set -u ;\
	pip install local_imports/voltha-protos/dist/*.tar.gz
endif

clean:
	rm -rf local_imports
	find . -name '*.pyc' | xargs rm -f

distclean: clean
	rm -rf ${VENVDIR}

# end file
