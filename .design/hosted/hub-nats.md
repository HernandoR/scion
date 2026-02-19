# Hub NATS Event Publishing

## Overview

The Scion Hub is the source of truth for all state changes — agent CRUD, status transitions, grove operations, and broker connectivity. The web frontend's real-time pipeline (M7/M8) consumes events from NATS via an SSE bridge, but nothing currently publishes to NATS. This document specifies the Hub-side NATS event publisher.

The feature is opt-in: when NATS is not configured, the Hub operates exactly as it does today. When a NATS URL is provided, the Hub connects and publishes state-change events after successful database writes.

For the SSE/NATS bridge architecture and client-side subscription model, see `web-frontend-design.md` §12.

---

## Design Principles

1. **Fire-and-forget.** NATS publish failures are logged but never fail the HTTP request. The database write is the commit point; NATS is a best-effort notification layer.
2. **Publish after commit.** Events are published only after the store operation succeeds. This avoids notifying subscribers about changes that were rolled back.
3. **Dual-publish for status.** Agent status changes are published to both the agent-scoped subject (`agent.{id}.status`) and the grove-scoped subject (`grove.{groveId}.agent.status`). This allows grove-level subscribers to receive lightweight updates without per-agent subscriptions.
4. **Subject hierarchy is the filter.** The Hub does not implement weight-class filtering. The subject hierarchy itself controls which subscribers receive which events. Heavy payloads (harness output) are published only to agent-scoped subjects; lightweight/medium events are published to grove-scoped subjects.
5. **No NATS dependency for startup.** The Hub starts and serves API requests even if NATS is unavailable. NATS connection is established asynchronously and reconnects automatically.

---

## Configuration

NATS publishing is controlled by a single enablement gate: if a NATS server URL is provided, publishing is enabled.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SCION_SERVER_NATS_URL` | Comma-separated NATS server URLs (e.g., `nats://localhost:4222`) | (empty — disabled) |
| `SCION_SERVER_NATS_TOKEN` | Authentication token for NATS | (empty) |

### YAML Configuration (`settings.yaml` or `server.yaml`)

```yaml
server:
  nats:
    url: "nats://localhost:4222"
    token: ""
```

### CLI Flag

```
scion server start --enable-hub --nats-url nats://localhost:4222
```

### GlobalConfig Addition

```go
// In pkg/config/hub_config.go

type NATSConfig struct {
    // URL is a comma-separated list of NATS server URLs.
    // If empty, NATS event publishing is disabled.
    URL   string `json:"url" yaml:"url" koanf:"url"`
    // Token is the authentication token for NATS.
    Token string `json:"token" yaml:"token" koanf:"token"`
}

// Added to GlobalConfig:
type GlobalConfig struct {
    // ... existing fields ...
    NATS NATSConfig `json:"nats" yaml:"nats" koanf:"nats"`
}
```

### hub.ServerConfig Addition

```go
// In pkg/hub/server.go

type ServerConfig struct {
    // ... existing fields ...

    // NATSUrl is a comma-separated list of NATS server URLs.
    // If non-empty, NATS event publishing is enabled.
    NATSUrl   string
    // NATSToken is the authentication token for NATS.
    NATSToken string
}
```

---

## Architecture

### Component: `EventPublisher`

A new service in `pkg/hub/events.go` that owns the NATS connection and provides typed publish methods. The publisher is injected into the `Server` struct following the same pattern as `storage`, `secretBackend`, etc.

```
┌─────────────┐        ┌──────────────────┐        ┌────────────┐
│  Handler    │──────►  │  EventPublisher  │──────►  │   NATS     │
│ (after DB   │ publish │                  │  pub    │   Server   │
│  commit)    │         │  - nats.Conn     │         │            │
│             │         │  - Publish()     │         │            │
└─────────────┘         └──────────────────┘         └────────────┘
```

### Interface

```go
// EventPublisher publishes state-change events to NATS.
// The zero-value (nil) is safe to call — all methods are no-ops when
// the publisher is nil, allowing handlers to call unconditionally.
type EventPublisher interface {
    // PublishAgentStatus publishes an agent status change to both
    // agent-scoped and grove-scoped subjects (dual-publish).
    PublishAgentStatus(ctx context.Context, agent *store.Agent)

    // PublishAgentCreated publishes an agent-created event.
    PublishAgentCreated(ctx context.Context, agent *store.Agent)

    // PublishAgentDeleted publishes an agent-deleted event.
    PublishAgentDeleted(ctx context.Context, agentID, groveID string)

    // PublishGroveUpdated publishes a grove metadata change.
    PublishGroveUpdated(ctx context.Context, grove *store.Grove)

    // PublishGroveDeleted publishes a grove deletion.
    PublishGroveDeleted(ctx context.Context, groveID string)

    // PublishBrokerStatus publishes a broker status change.
    PublishBrokerStatus(ctx context.Context, brokerID, status string)

    // PublishBrokerConnected publishes a broker-connected event
    // for each grove the broker serves.
    PublishBrokerConnected(ctx context.Context, brokerID, brokerName string, groveIDs []string)

    // PublishBrokerDisconnected publishes a broker-disconnected event
    // for each grove the broker served.
    PublishBrokerDisconnected(ctx context.Context, brokerID string, groveIDs []string)

    // Close drains the NATS connection gracefully.
    Close()
}
```

### Nil-safe Pattern

Handlers call `s.events.PublishAgentStatus(...)` unconditionally. When NATS is disabled, `s.events` is nil and the methods are defined on a nil receiver that returns immediately:

```go
func (p *NATSEventPublisher) PublishAgentStatus(ctx context.Context, agent *store.Agent) {
    if p == nil {
        return
    }
    // ... publish logic
}
```

This avoids `if s.events != nil` guards throughout the handlers.

---

## Subject Hierarchy & Message Formats

All payloads are JSON. Timestamps use RFC 3339.

### Grove-Scoped Subjects

These reach grove-level subscribers (`grove.{groveId}.>`).

| Subject | Trigger | Payload |
|---------|---------|---------|
| `grove.{groveId}.agent.status` | Agent status change | `AgentStatusEvent` |
| `grove.{groveId}.agent.created` | Agent created | `AgentCreatedEvent` |
| `grove.{groveId}.agent.deleted` | Agent deleted | `AgentDeletedEvent` |
| `grove.{groveId}.updated` | Grove metadata change | `GroveUpdatedEvent` |
| `grove.{groveId}.broker.connected` | Broker joined grove | `BrokerGroveEvent` |
| `grove.{groveId}.broker.disconnected` | Broker left grove | `BrokerGroveEvent` |

### Agent-Scoped Subjects

These reach agent-level subscribers (`agent.{agentId}.>`).

| Subject | Trigger | Payload |
|---------|---------|---------|
| `agent.{agentId}.status` | Agent status change | `AgentStatusEvent` |
| `agent.{agentId}.created` | Agent created | `AgentCreatedEvent` |
| `agent.{agentId}.deleted` | Agent deleted | `AgentDeletedEvent` |

### Broker-Scoped Subjects

| Subject | Trigger | Payload |
|---------|---------|---------|
| `broker.{brokerId}.status` | Broker heartbeat / status change | `BrokerStatusEvent` |

### Message Types

```go
// AgentStatusEvent is published on agent status transitions.
// Published to both grove.{groveId}.agent.status and agent.{agentId}.status.
type AgentStatusEvent struct {
    AgentID         string `json:"agentId"`
    Status          string `json:"status"`
    SessionStatus   string `json:"sessionStatus,omitempty"`
    ContainerStatus string `json:"containerStatus,omitempty"`
    Timestamp       string `json:"timestamp"`
}

// AgentCreatedEvent is published when an agent is created.
type AgentCreatedEvent struct {
    AgentID  string `json:"agentId"`
    Name     string `json:"name"`
    Template string `json:"template,omitempty"`
    GroveID  string `json:"groveId"`
    Status   string `json:"status"`
}

// AgentDeletedEvent is published when an agent is deleted.
type AgentDeletedEvent struct {
    AgentID string `json:"agentId"`
}

// GroveUpdatedEvent is published when grove metadata changes.
type GroveUpdatedEvent struct {
    GroveID string            `json:"groveId"`
    Name    string            `json:"name,omitempty"`
    Labels  map[string]string `json:"labels,omitempty"`
}

// BrokerGroveEvent is published when a broker connects/disconnects from a grove.
type BrokerGroveEvent struct {
    BrokerID   string `json:"brokerId"`
    BrokerName string `json:"brokerName,omitempty"`
}

// BrokerStatusEvent is published on broker heartbeat or status changes.
type BrokerStatusEvent struct {
    BrokerID string `json:"brokerId"`
    Status   string `json:"status"`
}
```

---

## Handler Integration Points

Each handler calls the publisher **after** the store operation succeeds. The call is a single line appended to the success path.

### Agent Handlers (`handlers.go`)

| Handler | Line | Publish Call |
|---------|------|-------------|
| `createAgent()` | After `s.store.CreateAgent()` succeeds | `s.events.PublishAgentCreated(ctx, agent)` |
| `createGroveAgent()` | After `s.store.CreateAgent()` succeeds | `s.events.PublishAgentCreated(ctx, agent)` |
| `updateAgentStatus()` | After `s.store.UpdateAgent()` succeeds | `s.events.PublishAgentStatus(ctx, agent)` |
| `handleAgentLifecycle()` | After lifecycle action completes (start/stop/restart) | `s.events.PublishAgentStatus(ctx, agent)` |
| `deleteAgent()` | After `s.store.DeleteAgent()` succeeds | `s.events.PublishAgentDeleted(ctx, agentID, groveID)` |
| `deleteGroveAgent()` | After `s.store.DeleteAgent()` succeeds | `s.events.PublishAgentDeleted(ctx, agentID, groveID)` |

### Grove Handlers (`handlers.go`)

| Handler | Line | Publish Call |
|---------|------|-------------|
| `updateGrove()` | After `s.store.UpdateGrove()` succeeds | `s.events.PublishGroveUpdated(ctx, grove)` |
| `deleteGrove()` | After `s.store.DeleteGrove()` succeeds | `s.events.PublishGroveDeleted(ctx, groveID)` |

### Broker Handlers (`server.go`, `handlers_brokers.go`)

| Handler | Line | Publish Call |
|---------|------|-------------|
| `controlChannel.SetOnDisconnect` callback | After marking broker offline | `s.events.PublishBrokerDisconnected(ctx, brokerID, groveIDs)` |
| `markBrokerOnline()` | After marking broker online | `s.events.PublishBrokerConnected(ctx, brokerID, brokerName, groveIDs)` |
| `handleGroveRegister()` | After registering broker to grove | `s.events.PublishBrokerConnected(ctx, brokerID, brokerName, []string{groveID})` |

---

## Server Integration

### Server Struct

```go
type Server struct {
    // ... existing fields ...
    events EventPublisher // NATS event publisher (nil when NATS disabled)
}
```

### Initialization (in `New()`)

```go
// Initialize NATS event publisher if configured
if cfg.NATSUrl != "" {
    publisher, err := NewNATSEventPublisher(cfg.NATSUrl, cfg.NATSToken)
    if err != nil {
        slog.Warn("Failed to connect to NATS — events disabled", "error", err)
    } else {
        srv.events = publisher
        slog.Info("NATS event publisher enabled", "servers", cfg.NATSUrl)
    }
}
```

### Shutdown (in `Shutdown()`)

```go
// Close NATS event publisher
if s.events != nil {
    s.events.Close()
}
```

### Setter (for `cmd/server.go` initialization)

```go
func (s *Server) SetEventPublisher(ep EventPublisher) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.events = ep
}
```

---

## NATS Connection Management

The `NATSEventPublisher` implementation wraps `nats.Conn` with:

- **Connect options:** `nats.MaxReconnects(-1)` (unlimited reconnect), `nats.ReconnectWait(2s)`, `nats.Name("scion-hub")`.
- **Token auth:** `nats.Token(token)` when token is configured.
- **Disconnect handler:** Logs warnings on disconnect, info on reconnect.
- **Graceful drain:** `conn.Drain()` on `Close()` — flushes pending publishes before disconnecting.
- **No connection blocking:** `nats.Connect()` is called with `nats.RetryOnFailedConnect(true)` so the Hub starts even if NATS is unreachable. The nats.go client will keep retrying in the background.

```go
type NATSEventPublisher struct {
    conn *nats.Conn
}

func NewNATSEventPublisher(url, token string) (*NATSEventPublisher, error) {
    opts := []nats.Option{
        nats.Name("scion-hub"),
        nats.MaxReconnects(-1),
        nats.ReconnectWait(2 * time.Second),
        nats.RetryOnFailedConnect(true),
        nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
            if err != nil {
                slog.Warn("NATS disconnected", "error", err)
            }
        }),
        nats.ReconnectHandler(func(nc *nats.Conn) {
            slog.Info("NATS reconnected", "server", nc.ConnectedUrl())
        }),
    }
    if token != "" {
        opts = append(opts, nats.Token(token))
    }

    nc, err := nats.Connect(url, opts...)
    if err != nil {
        return nil, fmt.Errorf("nats connect: %w", err)
    }

    return &NATSEventPublisher{conn: nc}, nil
}
```

### Publish Helper

Each typed publish method serializes the event to JSON and calls `conn.Publish()`. Errors are logged but not returned.

```go
func (p *NATSEventPublisher) publish(subject string, event interface{}) {
    data, err := json.Marshal(event)
    if err != nil {
        slog.Error("Failed to marshal NATS event", "subject", subject, "error", err)
        return
    }
    if err := p.conn.Publish(subject, data); err != nil {
        slog.Error("Failed to publish NATS event", "subject", subject, "error", err)
    }
}
```

---

## `cmd/server.go` Integration

The server command creates the publisher and injects it into the Hub.

```go
// After creating Hub server
if enableHub && hubSrv != nil {
    // Initialize NATS event publisher if configured
    natsURL := cfg.NATS.URL
    if envURL := os.Getenv("SCION_SERVER_NATS_URL"); envURL != "" {
        natsURL = envURL
    }
    if natsURL != "" {
        natsToken := cfg.NATS.Token
        if envToken := os.Getenv("SCION_SERVER_NATS_TOKEN"); envToken != "" {
            natsToken = envToken
        }
        publisher, err := hub.NewNATSEventPublisher(natsURL, natsToken)
        if err != nil {
            log.Printf("Warning: NATS event publisher failed to initialize: %v", err)
        } else {
            hubSrv.SetEventPublisher(publisher)
            log.Printf("NATS event publisher enabled: %s", natsURL)
        }
    }
}
```

---

## Health Check Integration

The `/readyz` endpoint should report NATS connectivity when publishing is enabled.

```json
{
  "status": "healthy",
  "uptime": "1h2m3s",
  "nats": {
    "enabled": true,
    "connected": true
  }
}
```

When NATS is enabled but disconnected, `/readyz` continues to return 200 (NATS is best-effort), but the status field reflects the current state. This differs from the web frontend where NATS disconnection returns 503, because the Hub's primary function is the API — NATS is supplementary.

---

## Runtime Broker Agent Status Updates

When a Runtime Broker reports an agent status change via `PUT /api/v1/agents/{id}/status`, the Hub's `updateAgentStatus()` handler writes to the database and then calls `s.events.PublishAgentStatus()`. This is the primary real-time update path:

```
Runtime Broker → Hub API (updateAgentStatus) → Database → NATS → Web SSE → Browser
```

The broker does not publish to NATS directly. All NATS publishing is centralized in the Hub.

---

## Testing

### Unit Tests

- `TestEventPublisherNil` — Verify nil publisher methods don't panic.
- `TestPublishAgentStatus` — Verify correct subjects and payload for dual-publish.
- `TestPublishAgentCreated` — Verify grove-scoped and agent-scoped subjects.
- `TestPublishAgentDeleted` — Verify both subjects receive the event.
- `TestPublishGroveUpdated` — Verify grove subject and payload.
- `TestPublishBrokerConnected` — Verify per-grove broker events.

### Integration Tests

Use an embedded NATS server (`github.com/nats-io/nats-server/v2/server`) for testing:

```go
func startTestNATS(t *testing.T) (*server.Server, string) {
    opts := &server.Options{Port: -1} // Random port
    ns, err := server.NewServer(opts)
    require.NoError(t, err)
    ns.Start()
    t.Cleanup(ns.Shutdown)
    return ns, ns.ClientURL()
}
```

### Manual Testing

```bash
# Terminal 1: Start NATS
docker run -p 4222:4222 nats:latest

# Terminal 2: Subscribe to all events
nats sub ">"

# Terminal 3: Start Hub with NATS enabled
scion server start --enable-hub --enable-runtime-broker --dev-auth \
  --nats-url nats://localhost:4222

# Terminal 4: Create an agent and observe events
export SCION_DEV_TOKEN=<token>
scion agent start --name test-agent
# → NATS subscriber should show grove.{id}.agent.created
# → NATS subscriber should show grove.{id}.agent.status (status=running)
```

---

## Implementation Milestones

### Phase 1: Core Publisher

1. Add `NATSConfig` to `GlobalConfig` and `ServerConfig`.
2. Add `--nats-url` and `--nats-token` flags to `cmd/server.go`.
3. Add koanf env mappings for `SCION_SERVER_NATS_URL` and `SCION_SERVER_NATS_TOKEN`.
4. Implement `EventPublisher` interface and `NATSEventPublisher` in `pkg/hub/events.go`.
5. Add `events` field to `Server` struct with `SetEventPublisher()` setter.
6. Initialize publisher in `cmd/server.go` and inject into Hub.
7. Add `Close()` to shutdown path.
8. Unit tests with nil publisher and mock NATS.

### Phase 2: Handler Integration

1. Add publish calls to agent handlers: `createAgent`, `createGroveAgent`, `updateAgentStatus`, `handleAgentLifecycle`, `deleteAgent`, `deleteGroveAgent`.
2. Add publish calls to grove handlers: `updateGrove`, `deleteGrove`.
3. Add publish calls to broker handlers: `markBrokerOnline`, `controlChannel.SetOnDisconnect`, `handleGroveRegister`.
4. Integration tests with embedded NATS server.

### Phase 3: Health & Observability

1. Add NATS status to `/readyz` response.
2. Add structured logging for publish activity at debug level.
3. End-to-end manual testing with web frontend SSE.

---

## Dependencies

- `github.com/nats-io/nats.go` — Go NATS client (already a transitive dependency if used elsewhere, otherwise add to `go.mod`).
- `github.com/nats-io/nats-server/v2` — Embedded NATS server for tests only.

---

## Non-Goals

- **NATS JetStream / persistence.** The publisher uses core NATS pub/sub. Message persistence is not needed because the web frontend fetches the full state snapshot on load and SSE reconnects restart from the current state, not from a historical offset.
- **Agent harness event relay.** Heavy events (`agent.{id}.event`) from the harness status stream are not part of this design. Those require a separate pipeline from the Runtime Broker's status monitor to NATS. This design covers Hub-originated state changes only.
- **NATS as a message bus for inter-service communication.** NATS is used strictly for fan-out notifications to the web frontend. The Hub-to-Broker communication continues to use the existing HTTP/WebSocket control channel.
