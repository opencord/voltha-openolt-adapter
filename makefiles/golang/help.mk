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

$(if $(DEBUG),$(warning ENTER))

## -----------------------------------------------------------------------
## Intent: Display topic help
## -----------------------------------------------------------------------
help-summary ::
	@printf '  %-30s %s\n' 'lint-shell'\
	  'Run shellcheck on sources'
	@printf '  %-30s %s\n' 'lint-shell-help'\
	  'Show verbose target help'
  ifdef VERBOSE
	@$(MAKE) --no-print-directory lint-shell-help
  endif

## -----------------------------------------------------------------------
## Intent: Display extended topic help
## -----------------------------------------------------------------------
.PHONY: lint-shell-help
lint-shell-help ::
	@printf '  %-30s %s\n' 'lint-shell-all'\
	  'Shellcheck available sources'
	@printf '  %-30s %s\n' 'lint-shell-mod'\
	  'Shellcheck locally modified files (git status)'
	@printf '  %-30s %s\n' 'lint-shell-src'\
	  'Shellcheck files by path'

$(if $(DEBUG),$(warning LEAVE))

# [EOF]
