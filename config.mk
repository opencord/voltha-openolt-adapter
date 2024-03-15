# -*- makefile -*-
# -----------------------------------------------------------------------
# Copyright 2023-2024 Open Networking Foundation Contributors
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

##--------------------------------##
##---]  Disable lint targets  [---##
##--------------------------------##
# NO-LINT-GROOVY    := true#               # Note[1]
# NO-LINT-JJB       := true#               # Note[2]
# NO-LINT-JSON      := true#               # Note[1]
# NO-LINT-MAKEFILE  := true
# NO-LINT-PYTHON    := true#               # Note[1]
# NO-LINT-REUSE     := true                # License check
# NO-LINT-SHELL     := true#               # Note[1]
# NO-LINT-YAML      := true#               # Note[1]

# Note[1] - Plenty of source to cleanup
# Note[2] - No sources available

##---------------------------------##
##---] Conditional make logic  [---##
##---------------------------------##
USE_DOCKER_MK    := true

# [EOF]
