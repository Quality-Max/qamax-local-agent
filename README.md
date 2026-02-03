# qamax-agent

Cross-platform CLI for running [QualityMax](https://qamax.co) Playwright tests locally.

- **Run** as a daemon to poll and execute Playwright tests from QualityMax cloud
- **Capture** browser cookies for authenticated test scenarios
- **Authenticate** via browser-based OAuth login
- **Manage** projects and credentials locally

Single binary, no runtime dependencies (Node.js/npm required only for test execution).

## Install

### Download binary

Download the latest release for your platform from [Releases](https://github.com/Quality-Max/qamax-local-agent/releases):

```bash
# macOS Apple Silicon
curl -fsSL -o qamax-agent https://github.com/Quality-Max/qamax-local-agent/releases/download/v2.0.0/qamax-agent-darwin-arm64

# macOS Intel
curl -fsSL -o qamax-agent https://github.com/Quality-Max/qamax-local-agent/releases/download/v2.0.0/qamax-agent-darwin-amd64

# Linux x86_64
curl -fsSL -o qamax-agent https://github.com/Quality-Max/qamax-local-agent/releases/download/v2.0.0/qamax-agent-linux-amd64

chmod +x qamax-agent
sudo mv qamax-agent /usr/local/bin/
```

### Install script

```bash
./install.sh
```

### Build from source

Requires Go 1.22+:

```bash
git clone https://github.com/Quality-Max/qamax-local-agent.git
cd qamax-local-agent
make build
```

Cross-compile for all platforms:

```bash
make build-all
```

## Quick Start

```bash
qamax-agent login                                          # Authenticate via browser
qamax-agent projects                                       # List your projects
qamax-agent run --cloud-url https://app.qamax.co           # Start the agent daemon
```

## Commands

### `login`

Authenticate with QualityMax via browser OAuth. Opens your browser and saves the token to `~/.qamax/config.json`.

```bash
qamax-agent login                        # Default (port 9876)
qamax-agent login --port 8080            # Custom callback port
qamax-agent login --api-url URL          # Custom QualityMax URL
```

### `run`

Start the agent daemon to poll for and execute test assignments.

```bash
qamax-agent run --cloud-url https://app.qamax.co
qamax-agent run --cloud-url https://app.qamax.co --registration-secret SECRET
qamax-agent run --poll-interval 10 --heartbeat-interval 30
```

After the first successful registration, credentials are saved. Subsequent runs use saved values as defaults.

**Backward compatibility** — the old flag-based invocation still works:

```bash
qamax-agent --cloud-url https://app.qamax.co --registration-secret SECRET
```

### `capture`

Launch Chrome, navigate to a URL, wait for manual login, then capture all cookies and localStorage, and upload them as authentication data.

```bash
qamax-agent capture --url https://example.com --project-id ID --name "Production Auth"
qamax-agent capture --url https://example.com --project-id ID --name "Staging" --output cookies.json
```

Captures are stored as Playwright-compatible storage state JSON. Requires prior `qamax-agent login` and Google Chrome installed.

### `projects`

List available projects.

```bash
qamax-agent projects
```

### `status`

Show current authentication and agent registration status.

```bash
qamax-agent status
```

### `token`

Print the saved OAuth token to stdout (useful for piping).

```bash
qamax-agent token
qamax-agent token | pbcopy    # Copy to clipboard on macOS
```

### `logout`

Remove saved credentials.

```bash
qamax-agent logout
```

## Configuration

Config is stored at `~/.qamax/config.json` (mode `0600`):

```json
{
  "token": "eyJ...",
  "api_url": "https://app.qamax.co",
  "agent_id": "uuid",
  "api_key": "hex-key",
  "registration_secret": ""
}
```

| Field | Purpose |
|-------|---------|
| `token` | OAuth JWT from `login`, used by `capture` and `projects` |
| `api_url` | QualityMax server URL |
| `agent_id` / `api_key` | Agent daemon credentials, saved after first `run` registration |
| `registration_secret` | Server-side secret for agent registration |

## Running as a Service

See [INSTALLATION.md](INSTALLATION.md) for macOS LaunchAgent and Linux systemd setup instructions.

## Prerequisites

| Requirement | Used by |
|-------------|---------|
| Node.js + npm | `run` (Playwright test execution) |
| Google Chrome | `capture` (cookie extraction via CDP) |

## Security

- All communication uses HTTPS/TLS
- Config file permissions are restricted to `0600` (owner read/write only)
- Config directory permissions are `0700`
- HTTP response bodies are size-limited to prevent memory exhaustion
- Login callback validates request method and token length

## License

Apache-2.0 -- see [LICENSE](LICENSE).
