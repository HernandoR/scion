# Technical Research: OpenTelemetry and Data Availability in OpenCode

This document catalogs the telemetry data and events available within the OpenCode ecosystem to inform the design of a custom collector for filtering and forwarding.

## 1. LLM & Agent Telemetry (OpenTelemetry)

OpenCode leverages the Vercel AI SDK's experimental telemetry features. When enabled in the configuration (`experimental.openTelemetry: true`), the SDK generates standard OpenTelemetry spans.

### Data Sources
- **LLM Streaming**: `packages/opencode/src/session/llm.ts`
- **Agent Generation**: `packages/opencode/src/agent/agent.ts`

### Available Spans & Attributes
| Span Name | Key Attributes | Description |
|-----------|----------------|-------------|
| `ai.generateText` / `ai.streamText` | `ai.model.id`, `ai.model.provider`, `ai.prompt.tokens`, `ai.completion.tokens`, `ai.total.tokens` | Captures the full lifecycle of an LLM completion request, including token usage and model identification. |
| `ai.toolCall` | `ai.tool.name`, `ai.tool.id`, `ai.tool.input` | Recorded when an agent decides to invoke a tool. Includes the raw JSON input provided to the tool. |
| `ai.toolResult` | `ai.tool.name`, `ai.tool.id`, `ai.tool.output` | Recorded upon completion of a tool execution. Contains the result returned to the LLM. |

### Configuration
- **Toggle**: `experimental.openTelemetry` in `opencode.jsonc`.
- **Metadata**: Custom metadata such as `userId` (from `config.username`) is attached to these spans.
- **Exporting**: The CLI currently does not register a default `OTLPTraceExporter`. It relies on environment-level configuration (e.g., `OTEL_EXPORTER_OTLP_ENDPOINT`) or global OpenTelemetry SDK registration.

---

## 2. Infrastructure & Console Observability (Honeycomb)

The OpenCode Console (Cloudflare Workers) uses a tail consumer to process logs and forward metrics to Honeycomb.

### Data Source
- **Log Processor**: `packages/console/function/src/log-processor.ts`
- **Infra Definition**: `infra/console.ts`

### Collected Data
- **Request Context**: Method, URL, Content-Length, Source IP (`x-real-ip`).
- **Edge Metadata**: `cf.continent`, `cf.country`, `cf.city`, `cf.timezone`, etc.
- **Execution Metrics**: `wallTime` (duration), response status.
- **Custom Metrics**: The processor scans logs for the `_metric:` prefix. Any JSON following this prefix is parsed and merged into the telemetry event.

---

## 3. CLI Event Stream (Structured JSON)

The CLI provides a structured event stream that can be used for local collection and forwarding.

### Mechanism
Running the CLI with the `--format json` flag:
```bash
opencode run "message" --format json
```

### Event Types
- **`tool_use`**: Detailed record of tool invocation, status, and output.
- **`step_start` / `step_finish`**: Marks the boundaries of agentic "steps" or iterations.
- **`text`**: The textual response content from the model.
- **`error`**: Categorized errors (name, message, cause).

---

## 4. Internal Business Events (Bus)

OpenCode uses an internal event bus (`packages/opencode/src/bus.ts`) to propagate state changes.

### Event Categories
- **`message.*`**: Creation and updates of message parts.
- **`session.*`**: Session lifecycle events (created, updated, idle, error).
- **`permission.*`**: Permission requests and responses.

Plugins can subscribe to these events and forward them to external collectors.

---

## 5. Summary for Custom Collector Design

To implement a custom collector that filters and forwards events, the following integration points are recommended:

1.  **OTLP Receiver**: Configure the custom collector as an OTLP endpoint and point the OpenCode CLI to it via `OTEL_EXPORTER_OTLP_ENDPOINT`.
2.  **Tail Consumer / Log Sink**: If running in a hosted environment (like Cloudflare), implement a processor similar to `log-processor.ts` to intercept `_metric:` logs.
3.  **CLI Wrapper**: For local developer environments, a wrapper could consume the `--format json` output of the CLI.
4.  **Plugin System**: Develop a core OpenCode plugin that implements the `event` hook to capture and forward Bus events directly.
