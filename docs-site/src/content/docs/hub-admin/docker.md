---
title: Hosting Scion with Docker
description: How to run the Scion Hub and Web Dashboard as a Docker container.
---

This guide walks you through running the Scion Hub (including the Web Dashboard) as a Docker container on any Linux host. It covers building the image, configuring OAuth login, and starting the container.

## Prerequisites

- Docker (20.10+) installed and running on the host
- A public or internal hostname / IP address for the server (used for OAuth redirect URIs)
- OAuth credentials from at least one identity provider (Google or GitHub) — required because the web dashboard needs authentication

## Build the Docker Image

The Scion Hub image is built from the repository root. The image bundles the `scion` binary with the embedded web frontend.

```bash
# From the repository root
docker build \
  -f image-build/scion-hub/Dockerfile \
  --build-arg BASE_IMAGE=golang:1.23-bookworm \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t scion-hub:latest .
```

:::note
`BASE_IMAGE` must be an image that includes Go and a Debian/Ubuntu base. The multi-stage build first compiles the binary, then copies only the final artifact into the runtime image.
:::

## Configure OAuth Login

The web dashboard requires users to log in. Scion supports **Google** and **GitHub** as OAuth identity providers. You must configure at least one provider before starting the container.

### Create OAuth Credentials

#### Google

1. Open the [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials).
2. Click **Create Credentials → OAuth client ID** and choose **Web application**.
3. Add an **Authorized redirect URI**:
   ```
   https://<your-host>/auth/callback/google
   ```
4. Note the **Client ID** and **Client Secret**.

#### GitHub

1. Go to **GitHub Settings → Developer settings → OAuth Apps → New OAuth App**.
2. Set **Homepage URL** to `https://<your-host>`.
3. Set **Authorization callback URL** to:
   ```
   https://<your-host>/auth/callback/github
   ```
4. Note the **Client ID** and **Client Secret**.

### Prepare an Environment File

Create a file (e.g., `scion-hub.env`) to hold your secrets. **Do not commit this file to source control.**

```bash
# scion-hub.env

# --- OAuth (configure at least one provider) ---
SCION_SERVER_OAUTH_WEB_GOOGLE_CLIENTID=your-google-client-id
SCION_SERVER_OAUTH_WEB_GOOGLE_CLIENTSECRET=your-google-client-secret

SCION_SERVER_OAUTH_WEB_GITHUB_CLIENTID=your-github-client-id
SCION_SERVER_OAUTH_WEB_GITHUB_CLIENTSECRET=your-github-client-secret

# CLI OAuth (optional — needed only if users will run `scion auth login`)
SCION_SERVER_OAUTH_CLI_GOOGLE_CLIENTID=your-google-client-id
SCION_SERVER_OAUTH_CLI_GOOGLE_CLIENTSECRET=your-google-client-secret

# --- Web session secret (use a long random string) ---
SCION_SERVER_WEB_SESSION_SECRET=replace-with-a-long-random-string

# --- (Optional) Restrict login to specific email domains ---
# SCION_SERVER_AUTH_AUTHORIZEDDOMAINS=example.com,mycompany.org

# --- (Optional) Bootstrap first admin user(s) ---
# SCION_SERVER_ADMIN_EMAILS=admin@example.com
```

Generate a random session secret with:
```bash
openssl rand -hex 32
```

## Run the Container

```bash
docker run -d \
  --name scion-hub \
  --env-file scion-hub.env \
  -p 8080:8080 \
  -p 9800:9800 \
  -v scion-hub-data:/home/scion/.scion \
  scion-hub:latest \
  server start \
    --foreground \
    --production \
    --enable-hub \
    --enable-runtime-broker \
    --enable-web \
    --web-port 8080 \
    --runtime-broker-port 9800 \
    --base-url "https://<your-host>" \
    --auto-provide
```

| Flag | Purpose |
|---|---|
| `--production` | Binds to `0.0.0.0` and disables dev-auth |
| `--enable-hub` | Starts the Hub API |
| `--enable-runtime-broker` | Starts the local Runtime Broker |
| `--enable-web` | Serves the Web Dashboard on `--web-port` |
| `--base-url` | Public URL used to build OAuth redirect URIs |
| `--auto-provide` | Automatically registers the co-located broker as a provider |

The named volume `scion-hub-data` persists the SQLite database and any local storage under `/home/scion/.scion`.

## Logging in via the Web Dashboard

1. Open `https://<your-host>` in your browser.
2. You will be redirected to the login page.
3. Click **Sign in with Google** or **Sign in with GitHub**.
4. Complete the OAuth flow. On first login the account is created automatically.

If you supplied `--admin-emails` (or `SCION_SERVER_ADMIN_EMAILS`), the matching user is promoted to admin immediately. Otherwise, promote the first admin manually via the CLI after logging in:

```bash
scion --hub https://<your-host> hub user promote <email> admin
```

## Connecting the CLI

After the hub is running, point the `scion` CLI at it from any machine:

```bash
scion auth login --hub https://<your-host>
```

This opens a browser to complete the OAuth flow and stores credentials in `~/.scion/config.json`.

## Using a Compose File

For convenience, here is a minimal `docker-compose.yml`:

```yaml
services:
  scion-hub:
    image: scion-hub:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "9800:9800"
    env_file:
      - scion-hub.env
    volumes:
      - scion-hub-data:/home/scion/.scion
    command:
      - server
      - start
      - --foreground
      - --production
      - --enable-hub
      - --enable-runtime-broker
      - --enable-web
      - --web-port=8080
      - --runtime-broker-port=9800
      - --base-url=https://<your-host>
      - --auto-provide

volumes:
  scion-hub-data:
```

Start with:
```bash
docker compose up -d
```

## Health Checks

The Hub exposes two HTTP health check endpoints:

| Endpoint | Purpose |
|---|---|
| `GET /healthz` | Liveness — returns `200 OK` when the process is running |
| `GET /readyz` | Readiness — also verifies database connectivity |

Add a Docker health check to the compose service:

```yaml
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
```

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Login redirects back to the login page with an error | OAuth redirect URI mismatch — ensure the URI in the provider console exactly matches `https://<your-host>/auth/callback/<provider>` |
| `session secret` warning in logs | `SCION_SERVER_WEB_SESSION_SECRET` is not set; sessions will not survive restarts |
| Container exits immediately | Run without `-d` to see startup errors: `docker run --rm --env-file scion-hub.env scion-hub:latest server start ...` |
| Database errors on startup | Verify the volume is mounted and the `scion` user (UID 1000) can write to `/home/scion/.scion` |

## Next Steps

- [Authentication & Identity](/scion/hub-admin/auth/) — detailed OAuth and domain restriction options
- [Permissions](/scion/hub-admin/permissions/) — managing user roles
- [Hub Setup](/scion/hub-admin/hub-server/) — full configuration reference
