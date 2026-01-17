# Technical Research: Local OpenTelemetry Forwarder for Gemini CLI

## Overview

This document outlines the design and implementation of a local-running OpenTelemetry (OTEL) collector configured as a "forwarder." The primary goal is to provide a middleware layer between the Gemini CLI and telemetry backends (like Jaeger or Google Cloud) that can selectively filter, transform, and route specific observability data.

## Architecture

The forwarder acts as a sidecar or a local gateway. The Gemini CLI exports data using the OTLP protocol (typically over gRPC) to the local forwarder, which then processes the data before sending it to its final destination.

```mermaid
graph LR
    A[Gemini CLI] -- OTLP/gRPC --> B[Local OTEL Forwarder]
    B -- Filtered Traces --> C[Local Jaeger]
    B -- Aggregated Metrics/Logs --> D[Debug Console / File]
    B -- Specific Events --> E[Remote Backend (GCP/etc)]
```

### Components
- **Receiver**: OTLP receiver (gRPC/HTTP) listening on `localhost:4317`.
- **Processors**: 
    - `batch`: For efficient network utilization.
    - `filter`: To drop or keep specific events based on names or attributes.
    - `attributes`: To enrich or redact data (e.g., removing PII from prompts).
- **Exporters**:
    - `otlp`: To forward traces to Jaeger or remote collectors.
    - `debug`: For local inspection of logs and metrics.
    - `file`: To persist telemetry to disk.

## Collector Configuration

The following configuration demonstrates how to use the `filter` processor to manage Gemini CLI events.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: "localhost:4317"

processors:
  batch:
    timeout: 1s
  
  # Example: Filter to only keep security and performance-related logs
  filter/logs:
    error_mode: ignore
    logs:
      include:
        match_type: strict
        record_names:
          - gemini_cli.api_error
          - gemini_cli.api_response
          - gemini_cli.tool_call

exporters:
  otlp/jaeger:
    endpoint: "localhost:14317"
    tls:
      insecure: true
  debug:
    verbosity: detailed
  file:
    path: "./forwarder-output.log"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/jaeger]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
    logs:
      receivers: [otlp]
      processors: [batch, filter/logs]
      exporters: [file, debug]
```

## Telemetry Data Schema

A comprehensive list of events and metrics emitted by Gemini CLI that can be targeted by the forwarder.

### Log Events

| Event Name | Description | Key Attributes |
| :--- | :--- | :--- |
| `gemini_cli.config` | Startup configuration | `model`, `extensions`, `mcp_servers` |
| `gemini_cli.user_prompt` | User interaction | `prompt`, `prompt_length`, `auth_type` |
| `gemini_cli.tool_call` | Tool execution details | `function_name`, `duration_ms`, `success` |
| `gemini_cli.tool_output_truncated` | Truncation signals | `tool_name`, `original_content_length` |
| `gemini_cli.file_operation` | Filesystem activity | `operation`, `path`, `mimetype` |
| `gemini_cli.api_request` | LLM Request | `model`, `prompt_id` |
| `gemini_cli.api_response` | LLM Response | `input_token_count`, `output_token_count` |
| `gemini_cli.api_error` | API Failures | `error_type`, `status_code` |
| `gemini_cli.model_routing` | Routing decisions | `decision_model`, `routing_latency_ms` |
| `gemini_cli.agent.finish` | Agent run results | `turn_count`, `terminate_reason` |
| `gen_ai.client.inference.operation.details` | Semantic GenAI event | `gen_ai.request.model`, `gen_ai.usage` |

### Metrics

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `gemini_cli.session.count` | Counter | Total CLI startups |
| `gemini_cli.tool.call.latency` | Histogram | Execution time of tools |
| `gemini_cli.api.request.latency` | Histogram | LLM response latency |
| `gemini_cli.token.usage` | Counter | Aggregated token consumption |
| `gemini_cli.lines.changed` | Counter | Code modification volume |
| `gemini_cli.agent.turns` | Histogram | Complexity of agent workflows |
| `gemini_cli.performance.score` | Histogram | Composite system health score |
| `gen_ai.client.token.usage` | Histogram | Standardized token usage |

## Implementation Strategy

1. **Binary Management**: Use `otelcol-contrib` (as seen in `local_telemetry.js`) because it includes the necessary `filter` and `transform` processors not found in the core distribution.
2. **Dynamic Filtering**: The forwarder can be configured to drop `gemini_cli.user_prompt` events if a certain environment variable or setting is detected, ensuring data privacy for specific sessions.
3. **Correlation**: Ensure the forwarder preserves `session.id` and `prompt_id` attributes to allow for cross-trace/log correlation in the final backend.
