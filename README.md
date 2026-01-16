<div align="center">
 <h1>Jira to Microsoft Teams Webhook Proxy</h1>
 <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.18%2B-blue?logo=go&style=for-the-badge" /></a>
 <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge" /></a>
</div>
<br />
<br />

A lightweight, configurable webhook proxy written in Go that receives Jira issue notifications and forwards them as richly formatted, user-mention-enabled messages to specific Microsoft Teams channels.

This service acts as a middleman, translating raw Jira webhook data into structured and readable [Adaptive Cards](https://adaptivecards.io/) for Microsoft Teams, making it easier for teams to track Jira activity directly where they collaborate.

## Features

  * **Jira Webhook Consumer:** Receives and processes webhook events from Jira for issue creation, updates, and comments.
  * **Dynamic Channel Routing:** Forwards notifications to different MS Teams channels based on a `/:team` parameter in the webhook URL (e.g., `/platform`, `/runtime`, `/network`).
  * **Rich Teams Notifications:** Formats messages as Adaptive Cards, a significant improvement over plain text.
  * **User @Mentions:** Automatically @mentions the Jira issue's "Issuer" (Creator) and "Assignee" in the Teams notification, ensuring they see the update.
  * **Highly Configurable:** All target webhook URLs are managed via a simple `config.yml` file or can be overridden with environment variables.
  * **Containerized:** Includes a `Dockerfile` for easy deployment and testing with Docker.
  * **Health Check:** Provides a `/healthz` endpoint for Kubernetes readiness/liveness probes.

## How It Works

The data flow is simple and effective:

1.  **Jira Event:** An action occurs in Jira (e.g., an issue is updated).
2.  **Webhook Trigger:** Jira sends an HTTP POST request with a detailed JSON payload to this proxy service. The URL provided to Jira includes a team-specific path (e.g., `http://your-proxy-service/runtime`).
3.  **Proxy Service Processing:**
      * The service receives the request and binds the JSON to its internal Go structs.
      * It looks up the correct MS Teams webhook URL from `config.yml` based on the `/runtime` path parameter.
      * It extracts key information (summary, type, issuer, assignee, issue URL).
      * It constructs a detailed Adaptive Card JSON payload, including the @mention data for the relevant users.
4.  **Notification Delivery:** The proxy sends the final Adaptive Card payload to the configured Microsoft Teams Incoming Webhook URL.
5.  **Teams Renders Card:** Microsoft Teams receives the payload and renders a rich, interactive card with clickable mentions and buttons in the designated channel.

## Getting Started

### Prerequisites

  * **Go:** Version 1.18 or higher.
  * **Docker:** Recommended for running the application in a consistent environment.
  * **Jira Instance:** With administrative permissions to create webhooks.
  * **Microsoft Teams:** With permissions to add an "Incoming Webhook" connector to a channel.

### 1. Configure Microsoft Teams

For each Teams channel you want to send notifications to, you must create a unique **Incoming Webhook URL**:

1.  In Microsoft Teams, navigate to the target channel.
2.  Click the three dots (•••) next to the channel name and select **Connectors**.
3.  Search for **Incoming Webhook**, click **Add**, and then **Configure**.
4.  Give the webhook a name (e.g., "Jira Notifications") and optionally upload an icon.
5.  Click **Create**.
6.  **Copy the generated Webhook URL.** This is a sensitive URL that you will add to your `config.yml`.
7.  Repeat this process for each channel (e.g., one for the default route, one for the "runtime" team, etc.).

### 2\. Configure the Proxy Application (`config.yml`)

In the root of the project, create a `config.yml` file. This file tells the application where to send notifications.

```yaml
# config.yml
api:
  port: 8080 # The port on which the proxy service will listen.

# Holds the webhook URLs for Microsoft Teams.
msteams:
  # The default URL used if the webhook URL doesn't specify a team (e.g., http://your-proxy/)
  # or if the specified team is not runtime, platform, or network.
  url: "YOUR_DEFAULT_MS_TEAMS_WEBHOOK_URL_HERE"
  
  # URL for the "runtime" team (triggered by http://your-proxy/runtime).
  runtime_url: "YOUR_RUNTIME_TEAM_MS_TEAMS_WEBHOOK_URL_HERE"

  # URL for the "platform" team (triggered by http://your-proxy/platform).
  platform_url: "YOUR_PLATFORM_TEAM_MS_TEAMS_WEBHOOK_URL_HERE"

  # URL for the "network" team (triggered by http://your-proxy/network).
  network_url: "YOUR_NETWORK_TEAM_MS_TEAMS_WEBHOOK_URL_HERE"
```

You can also override these settings using environment variables with the `MYAPP_` prefix. For example:

  * `MYAPP_API_PORT=9000`
  * `MYAPP_MSTEAMS_URL="your-url-here"`
  * `MYAPP_MSTEAMS_RUNTIME_URL="your-runtime-url-here"`

### 3\. Run the Application

#### Using Docker (Recommended)

1.  **Build the Docker image:**

    ```bash
    docker build -t jira-msteams-proxy .
    ```

2.  **Run the container:**
    This command runs the proxy on port `8080` and mounts your local `config.yml` into the container's working directory (`/root/` in this example; adjust if your Dockerfile's `WORKDIR` is different).

    ```bash
    docker run --rm -p 8080:8080 \
      -v $(pwd)/config.yml:/root/config.yml \
      jira-msteams-proxy
    ```

#### Running Locally

1.  **Install dependencies:**
    ```bash
    go mod tidy
    ```
2.  **Run the application:**
    ```bash
    go run ./cmd/webhook-proxy/ api
    ```

### 4\. Configure Jira Webhook

1.  Navigate to your Jira project settings: **Project Settings \> Automation**.
2.  Click **Create rule**.
3.  **Name:** Give it a clear name, e.g., "MS Teams Notifications".
4.  **URL:** This is the URL to your running proxy service, including the team path.
      * Add Component `Send web request`
      * To send notifications for the "platform" team to their specific channel, at Webhook URL use:
        `http://<your_proxy_service_host_or_ip>:8080/comment/platform` and set `Issue data` as `Webhook body`.
      * To send notifications to the default channel, use:
        `http://<your_proxy_service_host_or_ip>:8080/default` (or just `/`)

## API Endpoints

| Method | Path                | Description                                        |
|--------|---------------------|----------------------------------------------------|
| `POST` | `/:team`            | Handles issue created/updated events for a specific team. |
| `POST` | `/`                 | Handles issue created/updated events for the default team. |
| `POST` | `/comment/:team`    | Handles issue comment events for a specific team.  |
| `POST` | `/comment`          | Handles issue comment events for the default team.   |
| `GET`  | `/healthz`          | Health check endpoint. Returns HTTP 204 No Content. |

## Testing

You can simulate a Jira webhook request using `curl`.

1.  **Create a `sample_jira_payload.json` file** with a realistic payload (you can get one from the Jira REST API or a webhook catcher like `webhook.site`). Ensure it contains populated `creator` and `assignee` objects with valid `emailAddress`, `name`, and `displayName` fields for testing mentions.

2.  **Run this `curl` command** to send the test payload to the "runtime" team endpoint:

    ```bash
    curl -X POST \
      -H "Content-Type: application/json" \
      -d @sample_jira_payload.json \
      http://localhost:8080/runtime
    ```

    This will trigger a notification in the MS Teams channel configured for `runtime_url` in your `config.yml`.
