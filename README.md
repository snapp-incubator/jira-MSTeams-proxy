<div align="center">
 <h1>Jira Webhook Proxy — MSTeams & Mattermost</h1>
 <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.25%2B-blue?logo=go&style=for-the-badge" /></a>
 <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge" /></a>
</div>
<br />

A lightweight webhook proxy that receives Jira issue notifications and forwards them concurrently to **Microsoft Teams** and **Mattermost**.

Each notification channel is a self-contained `Notifier` implementation — adding new channels requires only implementing a single interface.

## Features

- **Multi-channel fan-out** — every Jira event is sent to all configured notifiers concurrently via `errgroup`.
- **Microsoft Teams** — Adaptive Cards with `@mentions` for creator and assignee, team-based URL routing.
- **Mattermost** — Slack-compatible attachments with color bars, fields, and "View Issue" links.
- **Notifier interface** — clean abstraction (`Name()` + `Send()`); each channel owns its formatting and HTTP delivery.
- **Independent failures** — one channel failing does not block or affect the others.
- **Always 200 OK** — errors are logged with `[notifier_name]` prefix, never propagated to Jira.
- **Mattermost is opt-in** — set `mattermost.webhook` to enable; leave empty for MSTeams-only mode.
- **Configurable** — `config.yml` with environment variable overrides (`MYAPP_` prefix).
- **Containerized** — multi-stage Dockerfile, Helm chart for Kubernetes.
- **Health check** — `GET /healthz` returns `204 No Content`.

## Architecture

```
Jira Webhook POST /:team
       │
       ▼
  HandleJiraWebhook
  ├─ bind JiraRequest
  ├─ errgroup.Go ──► MSTeamsNotifier.Send()
  │                    ├─ resolve team URL (platform/network/runtime/default)
  │                    ├─ generate AdaptiveCard with @mentions
  │                    └─ POST to Teams Incoming Webhook
  └─ errgroup.Go ──► MattermostNotifier.Send()
                       ├─ build Slack-compatible attachment
                       ├─ green bar for issues, blue for comments
                       └─ POST to Mattermost Incoming Webhook
       │
       ▼
  return 200 OK (errors logged)
```

### Package Structure

```
internal/webhook-proxy/
├── handler/
│   └── proxy.go              # Channel-agnostic fan-out handler
├── notifier/
│   ├── notifier.go           # Notifier interface
│   ├── msteams.go            # MSTeams implementation + Adaptive Card generation
│   ├── msteams_types.go      # Adaptive Card struct types
│   ├── mattermost.go         # Mattermost implementation
│   ├── mattermost_types.go   # Slack-compatible attachment types
│   ├── msteams_test.go
│   └── mattermost_test.go
├── request/
│   └── request.go            # Jira webhook payload structs
└── cmd/
    ├── api/api.go            # Startup wiring + Echo routes
    └── integration/main.go   # Local integration test harness
```

### Notifier Interface

```go
type Notifier interface {
    Name() string
    Send(ctx context.Context, req *request.JiraRequest, isComment bool, team string) error
}
```

## Getting Started

### Prerequisites

- **Go** 1.25+
- **Docker** (optional, for containerized deployment)
- **Jira** instance with webhook permissions
- **Microsoft Teams** channel with an [Incoming Webhook](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook) connector
- **Mattermost** channel with an [Incoming Webhook](https://developers.mattermost.com/integrate/webhooks/incoming/) (optional)

### 1. Configure Microsoft Teams

For each Teams channel that should receive notifications, create an Incoming Webhook:

1. Open the target channel → click **⋯** → **Connectors** → **Incoming Webhook**.
2. Name it (e.g. "Jira Notifications"), optionally upload an icon, click **Create**.
3. Copy the generated webhook URL.
4. Repeat for each team/channel (default, platform, runtime, network).

### 2. Configure Mattermost (Optional)

1. Go to **Integrations** → **Incoming Webhooks** → **Add Incoming Webhook**.
2. Choose the target channel, name it, click **Save**.
3. Copy the generated webhook URL.

### 3. Configure the Application (`config.yml`)

```yaml
api:
  port: 8080

msteams:
  # Default Teams webhook (used when no team matches or team is empty)
  url: "YOUR_DEFAULT_MS_TEAMS_WEBHOOK_URL"
  # Team-specific URLs (triggered by /:team path parameter)
  runtime_url: "YOUR_RUNTIME_TEAM_WEBHOOK_URL"
  platform_url: "YOUR_PLATFORM_TEAM_WEBHOOK_URL"
  network_url: "YOUR_NETWORK_TEAM_WEBHOOK_URL"

mattermost:
  # Set to enable Mattermost notifications. Leave empty to disable.
  webhook: "YOUR_MATTERMOST_WEBHOOK_URL"
```

**Environment variable overrides** (prefix `MYAPP_`, underscores become dots):

| Variable | Example |
|----------|---------|
| `MYAPP_API_PORT` | `9000` |
| `MYAPP_MSTEAMS_URL` | `https://...` |
| `MYAPP_MSTEAMS_RUNTIME_URL` | `https://...` |
| `MYAPP_MSTEAMS_PLATFORM_URL` | `https://...` |
| `MYAPP_MSTEAMS_NETWORK_URL` | `https://...` |
| `MYAPP_MATTERMOST_WEBHOOK` | `https://...` |

### 4. Run the Application

#### Using Docker

```bash
docker build -t jira-webhook-proxy .
docker run --rm -p 8080:8080 \
  -v $(pwd)/config.yml:/app/config.yml \
  jira-webhook-proxy
```

#### Locally

```bash
go mod tidy
go run ./cmd/webhook-proxy api
```

### 5. Configure Jira Webhook

1. Go to **Project Settings** → **Automation** → **Create rule**.
2. Add trigger (e.g. "Issue created", "Issue updated", "Comment added").
3. Add action **Send web request**.
4. Set the **Webhook URL** to your proxy endpoint:

| Use case | URL |
|----------|-----|
| Issue create/update → default channel | `http://<host>:8080/` |
| Issue create/update → platform team | `http://<host>:8080/platform` |
| Comment → platform team | `http://<host>:8080/comment/platform` |

5. Set **Issue data** as the webhook body.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/:team` | Issue created/updated → all notifiers |
| `POST` | `/` | Issue created/updated → default team |
| `POST` | `/comment/:team` | Comment added → all notifiers |
| `POST` | `/comment` | Comment added → default team |
| `GET` | `/healthz` | Health check → `204 No Content` |

## Testing

### Unit Tests

```bash
go test ./... -v
```

Tests cover:
- **`notifier/msteams_test.go`** — Name, Send success/failure, team routing, empty URL skip.
- **`notifier/mattermost_test.go`** — Name, Send success/failure, comment color, payload format.
- **`handler/proxy_test.go`** — Fan-out to multiple notifiers, one-notifier-fails still returns 200, invalid body returns 400.

### Local Integration Test

The `cmd/integration` harness starts the proxy with mock receivers so you can test end-to-end without real webhook URLs.

```bash
# Terminal 1 — start proxy + mock MSTeams receiver (Mattermost → real Snapp webhook)
go run ./cmd/integration

# Terminal 2 — send test requests
curl -s -w "\n→ HTTP %{http_code}\n" \
  -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d @sample_jira_payload.json

# Issue to platform team
curl -s -w "\n→ HTTP %{http_code}\n" \
  -X POST http://localhost:8080/platform \
  -H "Content-Type: application/json" \
  -d @sample_jira_payload.json

# Comment to platform team (blue color in Mattermost)
curl -s -w "\n→ HTTP %{http_code}\n" \
  -X POST http://localhost:8080/comment/platform \
  -H "Content-Type: application/json" \
  -d @sample_jira_payload.json

# Invalid body (expect 400)
curl -s -w "\n→ HTTP %{http_code}\n" \
  -X POST http://localhost:8080/platform \
  -H "Content-Type: application/json" \
  -d '{bad json'
```

Expected output in Terminal 1:
```
📩 [msteams] POST /platform
   Body (961 bytes): {"type":"message","attachments":[...]}

📩 [mattermost] POST /
   Body (496 bytes): {"text":"","attachments":[{"color":"#36a64f","title":"🎯 PROJ-125",...}]}
```

## Deployment (Helm)

```bash
helm install jira-webhook-proxy ./deployments/webhook-proxy \
  --set jira_element_webhook_url="https://..." \
  --set service_desk_notification.runtime="https://..." \
  --set service_desk_notification.platform="https://..." \
  --set service_desk_notification.network="https://..." \
  --set mattermost.webhook="https://..."
```

## License

This project is licensed under the MIT License.
