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

ifeq ($(TAG),)
TAG := latest
endif

ifeq ($(TARGET_TAG),)
TARGET_TAG := latest
endif

ifneq ($(http_proxy)$(https_proxy),)
# Include proxies from the environment
DOCKER_PROXY_ARGS = \
       --build-arg http_proxy=$(http_proxy) \
       --build-arg https_proxy=$(https_proxy) \
       --build-arg ftp_proxy=$(ftp_proxy) \
       --build-arg no_proxy=$(no_proxy) \
       --build-arg HTTP_PROXY=$(HTTP_PROXY) \
       --build-arg HTTPS_PROXY=$(HTTPS_PROXY) \
       --build-arg FTP_PROXY=$(FTP_PROXY) \
       --build-arg NO_PROXY=$(NO_PROXY)
endif

DOCKER_BUILD_ARGS = \
	--build-arg TAG=$(TAG) \
	--build-arg REGISTRY=$(REGISTRY) \
	--build-arg REPOSITORY=$(REPOSITORY) \
	$(DOCKER_PROXY_ARGS) $(DOCKER_CACHE_ARG) \
	 --rm --force-rm \
	$(DOCKER_BUILD_EXTRA_ARGS)

DOCKER_IMAGE_LIST = \
	voltha-openolt-adapter-base \
	voltha-openolt-adapter

VENVDIR := venv-$(shell uname -s | tr '[:upper:]' '[:lower:]')

.PHONY: $(DIRS) $(DIRS_CLEAN) base openolt_adapter

# This should to be the first and default target in this Makefile
help:
	@echo "Usage: make [<target>]"
	@echo "where available targets are:"
	@echo
	@echo "build             : Build the adapter."
	@echo "clean             : Remove files created by the build and tests"
	@echo "distclean         : Remove venv directory"
	@echo "help              : Print this help"
	@echo "rebuild-venv      : Rebuild local Python virtualenv from scratch"
	@echo "venv              : Build local Python virtualenv if did not exist yet"
	@echo "containers        : Build all the docker containers"
	@echo "base              : Build a base docker image with a modern version of pip and requirements.txt installed"
	@echo "openolt_adapter   : Build the openolt adapter docker container"
	@echo "tag               : Tag a set of images"
	@echo "push              : Push the docker images to an external repository"
	@echo "pull              : Pull the docker images from a repository"
	@echo

build: containers

containers: base openolt_adapter

base:
ifdef LOCAL_PYVOLTHA
	mkdir -p pyvoltha/dist
	cp ../../pyvoltha/dist/*.tar.gz pyvoltha/dist/
	mkdir -p voltha-protos/dist
	cp ../../voltha-protos/dist/*.tar.gz voltha-protos/dist/
	docker build $(DOCKER_BUILD_ARGS) -t ${REGISTRY}${REPOSITORY}voltha-openolt-adapter-base:${TAG} -f docker/Dockerfile.base_local .
else
	docker build $(DOCKER_BUILD_ARGS) -t ${REGISTRY}${REPOSITORY}voltha-openolt-adapter-base:${TAG} -f docker/Dockerfile.base .
endif

openolt_adapter: base
	docker build $(DOCKER_BUILD_ARGS) -t ${REGISTRY}${REPOSITORY}voltha-openolt-adapter:${TAG} -f docker/Dockerfile.openolt_adapter .

tag: $(patsubst  %,%.tag,$(DOCKER_IMAGE_LIST))

push: tag $(patsubst  %,%.push,$(DOCKER_IMAGE_LIST))

pull: $(patsubst  %,%.pull,$(DOCKER_IMAGE_LIST))

%.tag:
	docker tag ${REGISTRY}${REPOSITORY}voltha-$(subst .tag,,$@):${TAG} ${TARGET_REGISTRY}${TARGET_REPOSITORY}voltha-$(subst .tag,,$@):${TARGET_TAG}

%.push:
	docker push ${TARGET_REGISTRY}${TARGET_REPOSITORY}voltha-$(subst .push,,$@):${TARGET_TAG}

%.pull:
	docker pull ${REGISTRY}${REPOSITORY}voltha-$(subst .pull,,$@):${TAG}

clean:
	rm -rf pyvoltha
	rm -rf voltha-protos
	find . -name '*.pyc' | xargs rm -f

distclean: clean
	rm -rf ${VENVDIR}

purge-venv:
	rm -fr ${VENVDIR}

rebuild-venv: purge-venv venv

venv: ${VENVDIR}/.built

${VENVDIR}/.built:
	@ virtualenv ${VENVDIR}
	@ . ${VENVDIR}/bin/activate && \
	    pip install --upgrade pip; \
	    if ! pip install -r requirements.txt; \
	    then \
	        echo "On MAC OS X, if the installation failed with an error \n'<openssl/opensslv.h>': file not found,"; \
	        echo "see the BUILD.md file for a workaround"; \
	    else \
	        uname -s > ${VENVDIR}/.built; \
	    fi

ifdef LOCAL_PYVOLTHA
	mkdir -p pyvoltha/dist
	cp ../../pyvoltha/dist/*.tar.gz pyvoltha/dist/
	mkdir -p voltha-protos/dist
	cp ../../voltha-protos/dist/*.tar.gz voltha-protos/dist/
	@ . ${VENVDIR}/bin/activate && \
	    pip install pyvoltha/dist/*.tar.gz && \
	    pip install voltha-protos/dist/*.tar.gz
endif

# end file
