# QualityMax Local Agent - Installation Guide

## Overview

The QualityMax Local Agent (`qamax-agent`) is a single binary CLI that:
- Runs as a daemon to poll and execute Playwright tests from QualityMax cloud
- Authenticates via browser-based OAuth login
- Captures browser cookies for authenticated test scenarios
- Manages projects and credentials locally

## Quick Start (macOS/Linux)

### Prerequisites

- Node.js and npm (for Playwright test execution)
- Google Chrome (for the `capture` command)

### Installation

1. **Run the installer:**
   ```bash
   cd local-agent
   ./install.sh
   ```

2. **Log in:**
   ```bash
   qamax-agent login
   ```

3. **Start the agent:**
   ```bash
   qamax-agent run --cloud-url https://app.qamax.co --registration-secret YOUR_SECRET
   ```

### Building from Source

Requires Go 1.22+:

```bash
cd local-agent/go
go build -o qamax-agent .
```

Cross-compile for all platforms:

```bash
cd local-agent/go
make build-all
```

## Commands

### `qamax-agent login`

Authenticate with QualityMax via browser OAuth.

```bash
qamax-agent login                    # Uses default port 9876
qamax-agent login --port 8080        # Custom callback port
qamax-agent login --api-url URL      # Custom QualityMax URL
```

Opens your browser to log in. The token is saved to `~/.qamax/config.json`.

### `qamax-agent run`

Start the agent daemon to poll for and execute test assignments.

```bash
qamax-agent run --cloud-url https://app.qamax.co
qamax-agent run --cloud-url https://app.qamax.co --registration-secret SECRET
qamax-agent run --poll-interval 10 --heartbeat-interval 30
```

After the first successful registration, credentials are saved to config. Subsequent runs will use saved values as defaults.

**Backward compatibility:** The old flag-based invocation still works:
```bash
qamax-agent --cloud-url https://app.qamax.co --registration-secret SECRET
```

### `qamax-agent capture`

Launch Chrome, navigate to a URL, wait for manual login, then capture cookies and upload them as authentication data.

```bash
qamax-agent capture https://example.com --project-id UUID --name "Production Auth"
qamax-agent capture https://example.com --project-id UUID --name "Staging Auth" --output cookies.json
```

Requires:
- Prior `qamax-agent login` (uses OAuth token for API upload)
- Google Chrome installed

### `qamax-agent projects`

List available projects.

```bash
qamax-agent projects
```

### `qamax-agent status`

Show current authentication and agent registration status.

```bash
qamax-agent status
```

### `qamax-agent token`

Print the saved OAuth token to stdout (useful for piping).

```bash
qamax-agent token
qamax-agent token | pbcopy    # Copy to clipboard on macOS
```

### `qamax-agent logout`

Remove saved credentials.

```bash
qamax-agent logout
```

## Configuration

Config is stored at `~/.qamax/config.json` (mode 0600):

```json
{
  "token": "eyJ...",
  "api_url": "https://app.qamax.co",
  "agent_id": "uuid",
  "api_key": "hex-key",
  "registration_secret": ""
}
```

- `token` — OAuth JWT from `login`, used by `capture` and `projects`
- `agent_id` / `api_key` — Agent daemon credentials, saved after first `run` registration
- Both auth flows coexist and serve different purposes

## Running as a Service

### macOS (LaunchAgent)

Create `~/Library/LaunchAgents/com.qamax.agent.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.qamax.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/YOUR_USERNAME/.qamax-agent/qamax-agent</string>
        <string>run</string>
        <string>--cloud-url</string>
        <string>https://app.qamax.co</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/Users/YOUR_USERNAME/.qamax-agent/logs/agent.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/YOUR_USERNAME/.qamax-agent/logs/agent.error.log</string>
</dict>
</plist>
```

Load the service:
```bash
launchctl load ~/Library/LaunchAgents/com.qamax.agent.plist
```

### Linux (systemd)

Create `/etc/systemd/system/qamax-agent.service`:

```ini
[Unit]
Description=QualityMax Local Agent
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/home/YOUR_USERNAME/.qamax-agent/qamax-agent run --cloud-url https://app.qamax.co
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable qamax-agent
sudo systemctl start qamax-agent
```

## Troubleshooting

### Agent fails to register

- Check internet connection
- Verify cloud URL is correct
- Verify registration secret matches server configuration
- Review logs for detailed error messages

### Login fails

- Ensure port 9876 is available (or use `--port` to specify another)
- Check that the QualityMax app URL is correct
- Try `qamax-agent login --api-url https://app.qamax.co`

### Capture fails

- Ensure Google Chrome is installed
- Ensure you are logged in (`qamax-agent login`)
- Check that the project ID is valid (`qamax-agent projects`)

### No test assignments received

- Verify agent is online in QualityMax dashboard (`qamax-agent status`)
- Ensure tests are assigned to agents in the UI
- Check polling interval (default: 5 seconds)

### Tests fail to execute

- Ensure Node.js and npm are installed
- Verify Playwright is available: `npx playwright --version`
- Check browser availability

## Security

- All communication uses HTTPS/TLS
- Config file permissions are restricted to 0600 (owner read/write only)
- Config directory permissions are 0700
- API key and OAuth token are stored locally only
- Artifacts (screenshots, videos) are base64 encoded during transmission
