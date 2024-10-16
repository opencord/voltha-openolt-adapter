# -*- makefile -*-
# -----------------------------------------------------------------------
# Copyright 2016-2024 Open Networking Foundation Contributors
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
# -----------------------------------------------------------------------
# SPDX-FileCopyrightText: 2016-2024 Open Networking Foundation Contributors
# SPDX-License-Identifier: Apache-2.0
# -----------------------------------------------------------------------

$(if $(DEBUG),$(warning ENTER))

.DEFAULT_GOAL := help

##-------------------##
##---]  GLOBALS  [---##
##-------------------##

RSYNC        ?= rsync
RSYNC-CMD    := $(RSYNC) -rv --checksum --archive

TOP         ?= .
MAKEDIR     ?= $(TOP)/makefiles

$(if $(VERBOSE),$(eval export VERBOSE=$(VERBOSE))) # visible to include(s)

##--------------------##
##---]  INCLUDES  [---##
##--------------------##
include config.mk#               # configure
include makefiles/include.mk     # top level include

ifdef LOCAL_LINT
  include $(MAKEDIR)/lint/golang/sca.mk
endif

##------------------##
##---]  MACROS  [---##
##------------------##

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
DOCKER_TAG               ?= ${VERSION}$(shell [[ ${DOCKER_LABEL_VCS_DIRTY} == "true" ]] && echo "-dirty" || true)
ADAPTER_IMAGENAME        := ${DOCKER_REGISTRY}${DOCKER_REPOSITORY}voltha-openolt-adapter:${DOCKER_TAG}
DOCKER_TARGET            ?= prod
TYPE                     ?= minimal

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

# tool containers
VOLTHA_TOOLS_VERSION ?= 3.1.0

## TODO: Verify / migrate to repo:onf-make
# GO                = docker run --rm --user $$(id -u):$$(id -g) -v ${CURDIR}:/app $(shell test -t 0 && echo "-it") -v gocache:/.cache -v gocache-${VOLTHA_TOOLS_VERSION}:/go/pkg voltha/voltha-ci-tools:${VOLTHA_TOOLS_VERSION}-golang go
# GO_JUNIT_REPORT   = docker run --rm --user $$(id -u):$$(id -g) -v ${CURDIR}:/app -i voltha/voltha-ci-tools:${VOLTHA_TOOLS_VERSION}-go-junit-report go-junit-report
# GOCOVER_COBERTURA = docker run --rm --user $$(id -u):$$(id -g) -v ${CURDIR}:/app/src/github.com/opencord/voltha-openolt-adapter -i voltha/voltha-ci-tools:${VOLTHA_TOOLS_VERSION}-gocover-cobertura gocover-cobertura
# GOLANGCI_LINT     = docker run --rm --user $$(id -u):$$(id -g) -v ${CURDIR}:/app $(shell test -t 0 && echo "-it") -v gocache:/.cache -v gocache-${VOLTHA_TOOLS_VERSION}:/go/pkg voltha/voltha-ci-tools:${VOLTHA_TOOLS_VERSION}-golangci-lint golangci-lint
# HADOLINT          = docker run --rm --user $$(id -u):$$(id -g) -v ${CURDIR}:/app $(shell test -t 0 && echo "-it") voltha/voltha-ci-tools:${VOLTHA_TOOLS_VERSION}-hadolint hadolint

.PHONY: docker-build help

## -----------------------------------------------------------------------
## Intent: Local Development Helpers.  When LOCAL_PROTOS= is defined
##         copy in locally modified repo:voltha-protos for a building.
## -----------------------------------------------------------------------
## Usage : make build LOCAL_LIB_GO=/path/to/my/voltha-protos
## -----------------------------------------------------------------------
.PHONY: local-protos
protos-v5 := vendor/github.com/opencord/voltha-protos/v5/go

local-protos:

  ifdef LOCAL_PROTOS
	$(call banner-enter,$@)

	$(RM) -r "$(protos-v5)"
	mkdir -p "$(protos-v5)"
	$(RSYNC-CMD) "${LOCAL_PROTOS}/go/." "$(protos-v5)/."
	$(RM) -r "$(protos-v5)/vendor"

	$(call banner-leave,$@)
  endif # LOCAL_PROTOS=

## -----------------------------------------------------------------------
## Intent: Local Development Helpers.  When LOCAL_LIB_GO= is defined
##         copy in locally modified repo:voltha-protos for a building.
## -----------------------------------------------------------------------
## Usage : make build LOCAL_LIB_GO=/path/to/my/voltha-lib-go
## -----------------------------------------------------------------------
.PHONY: local-lib-go
lib-go-v7 := vendor/github.com/opencord/voltha-lib-go/v7

local-lib-go:

  ifdef LOCAL_LIB_GO
	$(call banner-enter,$@)

	mkdir -p "$(lib-go-v7)/pkg"
	$(RSYNC-CMD) "${LOCAL_LIB_GO}/pkg/." "$(lib-go-v7)/pkg/."

	$(call banner-leave,$@)
  endif # LOCAL_LIB_GO=

## -----------------------------------------------------------------------
## Docker targets
## -----------------------------------------------------------------------
build: docker-build ## Alias for 'docker build'

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
docker-build: local-protos local-lib-go ## Build openolt adapter docker image (set BUILD_PROFILED=true to also build the profiled image)
	docker build $(DOCKER_BUILD_ARGS) --platform=linux/amd64 --target=${DOCKER_TARGET} --build-arg CGO_PARAMETER=0 -t ${ADAPTER_IMAGENAME} -f docker/Dockerfile.openolt .
ifdef BUILD_PROFILED
	docker build $(DOCKER_BUILD_ARGS) --platform=linux/amd64 --target=dev --build-arg CGO_PARAMETER=1 --build-arg EXTRA_GO_BUILD_TAGS="-tags profile" -t ${ADAPTER_IMAGENAME}-profile -f docker/Dockerfile.openolt .
endif
ifdef BUILD_RACE
	docker build $(DOCKER_BUILD_ARGS) --platform=linux/amd64 --target=dev --build-arg CGO_PARAMETER=1 --build-arg EXTRA_GO_BUILD_TAGS="-race" -t ${ADAPTER_IMAGENAME}-rd -f docker/Dockerfile.openolt .
endif

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
docker-push: ## Push the docker images to an external repository
	docker push ${ADAPTER_IMAGENAME}
ifdef BUILD_PROFILED
	docker push ${ADAPTER_IMAGENAME}-profile
endif
ifdef BUILD_RACE
	docker push ${ADAPTER_IMAGENAME}-rd
endif

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
docker-kind-load: ## Load docker images into a KinD cluster
	@if [ "`kind get clusters | grep voltha-$(TYPE)`" = '' ]; then echo "no voltha-$(TYPE) cluster found" && exit 1; fi
	kind load docker-image ${ADAPTER_IMAGENAME} --name=voltha-$(TYPE) --nodes $(shell kubectl get nodes --template='{{range .items}}{{.metadata.name}},{{end}}' | sed 's/,$$//')

## -----------------------------------------------------------------------
## lint and unit tests
## -----------------------------------------------------------------------
lint-dockerfile: ## Perform static analysis on Dockerfile
	@echo "Running Dockerfile lint check ..."
	@${HADOLINT} $$(find . -name "Dockerfile.*")
	@echo "Dockerfile lint check OK"

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
lint-mod: ## Verify the Go dependencies

	@echo "Running dependency check..."
	@${GO} mod verify

	@echo "Dependency check OK. Running vendor check..."
	@git status > /dev/null
	@git diff-index --quiet HEAD -- go.mod go.sum vendor || (echo "ERROR: Staged or modified files must be committed before running this test" && git status -- go.mod go.sum vendor && exit 1)
	@[[ `git ls-files --exclude-standard --others go.mod go.sum vendor` == "" ]] || (echo "ERROR: Untracked files must be cleaned up before running this test" && git status -- go.mod go.sum vendor && exit 1)

	$(MAKE) mod-update
#	${GO} mod tidy
#	${GO} mod vendor

	@git status > /dev/null
	@git diff-index --quiet HEAD -- go.mod go.sum vendor || (echo "ERROR: Modified files detected after running go mod tidy / go mod vendor" && git status -- go.mod go.sum vendor && git checkout -- go.mod go.sum vendor && exit 1)
	@[[ `git ls-files --exclude-standard --others go.mod go.sum vendor` == "" ]] || (echo "ERROR: Untracked files detected after running go mod tidy / go mod vendor" && git status -- go.mod go.sum vendor && git checkout -- go.mod go.sum vendor && exit 1)
	@echo "Vendor check OK."

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
lint: local-lib-go lint-mod lint-dockerfile

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
coverage-out := ./tests/results/go-test-coverage.out
coverage-res := ./tests/results/go-test-results.out
test: ## Run unit tests

	$(call banner-enter,$@)

	$(RM) -r tests/results
	@mkdir -p ./tests/results

	@$(if $(LOCAL_FIX_PERMS),chmod 777 tests/results)

	$(HIDE) $(MAKE) --no-print-directory test-go-coverage
	$(HIDE) $(MAKE) --no-print-directory test-junit
	$(HIDE) $(MAKE) --no-print-directory test-cobertura

#	${GOCOVER_COBERTURA} < $(coverage-out) \
#	    > ./tests/results/go-test-coverage.xml

	@$(if $(LOCAL_FIX_PERMS),chmod 775 tests/results) # yes this may not run

	$(call banner-leave,$@)

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
test-go-coverage:
	$(call banner-enter,$@)
	@$(if $(LOCAL_FIX_PERMS),chmod 777 tests/results)
	( ${GO} test -mod=vendor -v -coverprofile $(coverage-out) -covermode count ./... 2>&1 ) \
	  | tee $(coverage-res)
	@$(if $(LOCAL_FIX_PERMS),chmod 775 tests/results)
	$(call banner-leave,$@)

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
test-junit:
	$(call banner-enter,$@)
	@$(if $(LOCAL_FIX_PERMS),chmod 777 tests/results)
	${GO_JUNIT_REPORT} < $(coverage-res) \
	    > ./tests/results/go-test-results.xml
	@$(if $(LOCAL_FIX_PERMS),chmod 775 tests/results)
	$(call banner-leave,$@)

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
test-cobertura:
	$(call banner-enter,$@)
	@$(if $(LOCAL_FIX_PERMS),chmod 777 tests/results)
	${GOCOVER_COBERTURA} < $(coverage-out) \
	    > ./tests/results/go-test-coverage.xml
	@$(if $(LOCAL_FIX_PERMS),chmod 775 tests/results)
	$(call banner-leave,$@)

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
help ::
	@echo '[TEST: Coverage report]'
	@echo '  test-go-coverage       Generate a coverage report for vendor/'
	@echo '  test-junit             Digest go coverage, generate junit'
	@echo '  test-cobertura         Digest coverage and junit reports'

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
sca:
	$(call banner-enter,$@)

	@$(RM) -r ./sca-report
	@mkdir -p ./sca-report
	@echo "Running static code analysis..."
	@${GOLANGCI_LINT} run -vv --out-format junit-xml ./... | tee ./sca-report/sca-report.xml
	@echo ""
	@echo "Static code analysis OK"

	$(call banner-leave,$@)

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
clean :: distclean
sterile :: clean distclean

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
distclean:
	$(RM) -r ./sca-report

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
.PHONY: mod-update
mod-update: mod-tidy mod-vendor

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
.PHONY: mod-tidy
mod-tidy:
	$(call banner-enter,$@)
	${GO} mod tidy
	$(call banner-leave,$@)

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
.PHONY: mod-vendor
mod-vendor:
	$(call banner-enter,$@)
	@$(if $(LOCAL_FIX_PERMS),chmod 777 .)
	${GO} mod vendor
	@$(if $(LOCAL_FIX_PERMS),chmod 755 .)
	$(call banner-leave,$@)

## -----------------------------------------------------------------------
## Intent: Render README.md for interactive viewing.
## -----------------------------------------------------------------------
view :
	pandoc README.md | lynx -stdin

## -----------------------------------------------------------------------
## Intent: Display supported makefile targets
## -----------------------------------------------------------------------
help ::
	@printf '  %-33.33s %s\n' 'local-help' \
	  'Show extended help for local build targets'
	@printf '  %-33.33s %s\n' 'sca' 'golang: static code analysis'
	@printf '  %-33.33s %s\n' 'view' \
	  'Render markdown (.md) for interactive viewing'

help ::
	@echo
	@echo '[MOD UPDATE]'
	@echo '  mod-update'
	@echo '    LOCAL_FIX_PERMS=1    Hack to fix docker access problems'
	@echo '  mod-tidy'
	@echo '  mod-vendor'

local-help :
	@echo
	@echo '[LOCAL: dev]'

	@printf '  %-33.33s %s\n' 'local-protos' \
	  'Copy a local dev version of voltha-protos into vendor/'
	@printf '    %s\n' '% $(MAKE) local-protos LOCAL_PROTOS={path}'

	@echo
	@printf '  %-33.33s %s\n' 'local-lib-go' \
	  'Copy a local dev version of voltha-lib-go into vendor/'
	@printf '    %s\n' '% $(MAKE) local-lib-go LOCAL_LIB_GO={path}'

# Thought: Use "$(MAKE) LOCAL=1" to wrap
#          "make local-protos local-lib-go build"

# [EOF]
