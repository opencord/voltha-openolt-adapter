# -*- makefile -*-
# -----------------------------------------------------------------------
# Copyright 2022-2024 Open Networking Foundation (ONF) and the ONF Contributors
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

## -----------------------------------------------------------------------
## -----------------------------------------------------------------------
.PHONY: help
help :: help-usage help-by-target

help-usage :
	@echo "Usage: $(MAKE) [options] [target] ..."
	@echo
	@echo "[LINT]"
	@echo "  lint-mod               Verify the integrity of the 'mod' files"
	@echo "  sca                    Run static code analysis with the golangci-lint tool"
	@echo
	@echo "[CLEAN]"
	@echo "  clean                  Remove files created by the build"
	@echo "  distclean              Remove build and testing artifacts and reports"
	@echo "[RELEASE]"
	@echo "  mod-update             go.mod with released versions, update go.sum and vendor/ files"

help-by-target:
	@echo
	@grep --no-filename '^[[:alpha:]_-]*:.* ##' $(MAKEFILE_LIST) \
	    | sort \
	    | awk 'BEGIN {FS=":.* ## "}; {printf "%-25s : %s\n", $$1, $$2};'

# [EOF]
