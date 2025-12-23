# Scion LLM-Agnostic Design

This document outlines the strategy for making `scion` more LLM-agnostic, allowing it to support multiple types of agents (e.g., Gemini, Claude) beyond its initial Gemini CLI focus.

## Current Gemini-Specific Coupling

- **Configuration Path**: Hardcoded use of `.gemini` directory in agent home.
- **Settings Format**: Hardcoded `settings.json` structure derived from Gemini CLI.
- **Environment Variables**: Use of `GEMINI_API_KEY` and `GEMINI_SANDBOX`.
- **Default Images**: Defaulting to `gemini-cli-sandbox`.
- **Status Files**: Hardcoded `.gemini-status.json`.
- **Aliases**: Default `.bashrc` includes `alias g="gemini"`.

## Proposed Prioritized Areas

### 1. Generalize Container Environment (High Priority)

Instead of hardcoding environment variables like `GEMINI_API_KEY`, `scion` should support a more flexible environment propagation system.

- **Action**: Move tool-specific environment variable names into `scion.json` or template metadata.
- **Action**: Rename `GEMINI_SANDBOX` to `SCION_RUNTIME`.

### 2. Abstract Config and Status Paths (High Priority)

Different agents expect configuration and write status to different locations.

- **Action**: In `scion.json`, allow specifying the path for the agent's main configuration directory and the status file location.
- **Action**: Update the manager to read status from the configured path instead of a hardcoded `.gemini-status.json`.

### 3. Template Refactoring (Medium Priority)

The current `default` template is essentially a "gemini-default" template.

- **Action**: Rename `default` to `gemini-default` (or keep as default but make it clear it's a provider-specific implementation).
- **Action**: Create a `claude-default` template structure.
- **Action**: Ensure `InitProject` can seed multiple provider-specific templates.

### 4. Image and Command Abstraction (Medium Priority)

- **Action**: The `scion start` command currently assumes the task is passed as arguments that the container entrypoint knows how to handle. This should be more explicit.
- **Action**: Allow `scion.json` to define the `entrypoint` or `cmd` wrapper if the image doesn't handle it.

### 5. Authentication Discovery (Low Priority)

- **Action**: Generalize `pkg/config/auth.go` to support multiple authentication types (e.g., `ANTHROPIC_API_KEY`).
- **Action**: Allow templates to define their own auth discovery logic or required environment variables.

## Implementation Phases

### Phase 1: Core Decoupling
- Update `scion.json` schema to include `env_map`, `config_dir`, and `status_file`.
- Modify `pkg/config` and `pkg/runtime` to respect these new settings.
- Rename generic "Gemini" terms in internal code (e.g., `GetGeminiSettings` -> `GetAgentSettings`).

### Phase 2: Multi-Provider Templates
- Update `scion grove init` to optionally take a provider type.
- Structure templates under `templates/<provider>/<type>`.

### Phase 3: Enhanced Human-in-the-Loop
- Generalize the status polling to support different status formats if necessary (though keeping a common `scion-status.json` format is preferred).
