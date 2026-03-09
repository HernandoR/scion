# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""ADK lifecycle callbacks that bridge to scion status reporting.

Maps ADK agent callbacks to scion status transitions by writing transient
states to $HOME/agent-info.json. Sticky states (WAITING_FOR_INPUT, COMPLETED)
are handled separately through the sciontool_status tool — not here.

All callbacks return None so they never interfere with ADK's execution flow.
"""

import logging

from . import sciontool

logger = logging.getLogger(__name__)


async def before_agent_callback(callback_context):
    """Agent starts processing — set status to THINKING."""
    sciontool.write_agent_status("THINKING")
    return None


async def before_tool_callback(tool, args, tool_context):
    """Tool about to execute — set status to EXECUTING."""
    sciontool.write_agent_status("EXECUTING")
    return None


async def after_tool_callback(tool, args, tool_context, tool_response):
    """Tool finished — agent resumes thinking."""
    sciontool.write_agent_status("THINKING")
    return None


async def after_agent_callback(callback_context):
    """Agent turn complete — set status to IDLE."""
    sciontool.write_agent_status("IDLE")
    return None
