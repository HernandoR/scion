# Claude Code OpenTelemetry Data Catalog

This document provides a technical research summary of the OpenTelemetry (OTel) data exported by Claude Code. This information is intended to inform the configuration of a custom OTel collector for filtering, sampling, and forwarding data.

## 1. Architecture Overview

Claude Code supports OpenTelemetry (OTel) for monitoring and observability, exporting metrics and events.

- **Exporters**: Supports OTLP via gRPC or HTTP.
- **Signals**: Metrics and Logs/Events.
- **Configuration**: Primarily via environment variables or managed settings files.

## 2. Configuration (Environment Variables)

Claude Code uses standard OTel environment variables along with harness-specific ones:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `CLAUDE_CODE_ENABLE_TELEMETRY` | Master toggle for telemetry | `false` |
| `OTEL_METRICS_EXPORTER` | Metrics exporter (e.g., `otlp`, `prometheus`, `console`) | `otlp` |
| `OTEL_LOGS_EXPORTER` | Logs/Events exporter (e.g., `otlp`, `console`) | `otlp` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | - |
| `OTEL_EXPORTER_OTLP_HEADERS` | Authentication headers for the collector | - |

## 3. Log Events (Structured Events)

Claude Code exports high-value structured events as OTel Log records.

| Event Name | Description | Key Attributes |
| :--- | :--- | :--- |
| `agent.user.prompt` | Capture of user input | `prompt` (Redacted by default) |
| `agent.tool.call` | Request to execute a tool | `tool_name`, `tool_input` |
| `agent.tool.result` | Outcome of a tool execution | `tool_name`, `success`, `duration_ms` |
| `gen_ai.api.request` | API call to LLM provider | `model`, `duration_ms` |
| `gen_ai.api.error` | API error occurred | `error_type`, `status_code` |
| `agent.tool.decision` | User decision on tool approval | `tool_name`, `decision` |

## 4. Metrics

Metrics are exported for aggregate analysis and cost monitoring.

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `claude_code.session.count` | Counter | Number of CLI sessions initiated |
| `claude_code.tokens.input` | Counter | Total input tokens used |
| `claude_code.tokens.output` | Counter | Total output tokens generated |
| `claude_code.cost.total` | Counter | Estimated total cost (USD) |
| `claude_code.lines.modified` | Counter | Total lines of code changed |
| `claude_code.tool.decisions` | Counter | Breakdown of tool approvals/rejections |
| `claude_code.active_time` | Histogram | Duration of active agent usage |
| `claude_code.pr.count` | Counter | Number of pull requests created |
| `claude_code.commit.count` | Counter | Number of commits generated |

## 5. Security and Privacy Defaults

- **Redaction**: User prompt content and tool details are redacted by default.
- **Opt-in**: Detailed logging of prompts requires explicit configuration.
- **PII Protection**: Sensitive data like API keys or raw file contents are excluded from telemetry.
