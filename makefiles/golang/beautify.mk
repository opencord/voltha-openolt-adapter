# -*- makefile -*-
# -----------------------------------------------------------------------
# Copyright 2022-2023 Open Networking Foundation (ONF) and the ONF Contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# -----------------------------------------------------------------------
# Intent: This makefile is used to apply standard formatting to golang src
# -----------------------------------------------------------------------

$(if $(DEBUG),$(warning ENTER))

##-------------------##
##---]  GLOBALS  [---##
##-------------------##
.PHONY: beautify-golang

# beautify :: beautify-golang # default target (?)

##-------------------##
##---]  TARGETS  [---##
##-------------------##

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
beautify-golang-args += -name '*.go'
beautify-golang-args += -not -path '*/vendor/*'
beautify-golang:
	find . $(beautify-golang-args) -print0\
	  | $(xargs-n1-clean) gofmt -e -s -w
	$(HIDE)git status

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
help ::
	@printf "  %-33.33s %s\n" 'beautify-golang' \
	  'Autoformat golang source using gofmt -e -s w'

# [EOF]
