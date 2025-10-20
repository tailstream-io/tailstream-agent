# Tailstream Agent

A lightweight Go agent that automatically discovers and ships logs to the Tailstream ingest API for real-time log analysis. Streams raw log lines as-is to the backend for parsing.

## ⚡ Quick Start

Choose your installation method:
- **Production/Always-on**: [One-liner installation](#one-liner-installation-recommended) - Installs as a system service
- **Ad-hoc/Testing**: [Stdin mode](#stdin-mode-pipe-any-log-source) - Pipe logs directly without installation

### One-Liner Installation (Recommended)

Install and set up the agent as a system service with a single command:

```bash
curl -fsSL https://install.tailstream.io | sudo bash
```

This will:
- Download the correct binary for your Linux architecture (x86_64/ARM64)
- Install to `/opt/tailstream/bin/tailstream-agent` with symlink at `/usr/local/bin/tailstream-agent`
- Create a dedicated `tailstream` user with ownership of `/opt/tailstream`
- Set up log file permissions (ACL preferred, group fallback)
- Set up a systemd service that starts on boot (runs as `tailstream` user)
- Enable the service (ready to start after configuration)

**Note:** Only the installation step requires root (`sudo`). After setup, the agent runs as a non-root user.

After installation, run the setup wizard:
```bash
sudo -u tailstream tailstream-agent
```

### Security & Permissions

> [!TIP]
> Root permissions are only required during installation.

 After installation, the Tailstream Agent runs as the dedicated `tailstream` user (created by the installer) and does NOT require root privileges for normal operation. This approach follows the principle of least privilege for improved security.

- **Install:** Requires root (`sudo`) to set up binaries, permissions, and the system service.
- **Run:** The agent runs as a non-root user (`tailstream`) with only the permissions needed to access log files.


### Manual Installation

1. **Download** the binary for your platform:
   - **Linux (x86_64)**: [tailstream-agent-linux-amd64](https://github.com/tailstream-io/tailstream-agent/releases/latest/download/tailstream-agent-linux-amd64)
   - **Linux (ARM64)**: [tailstream-agent-linux-arm64](https://github.com/tailstream-io/tailstream-agent/releases/latest/download/tailstream-agent-linux-arm64)
   - **macOS (ARM64)**: [tailstream-agent-darwin-arm64](https://github.com/tailstream-io/tailstream-agent/releases/latest/download/tailstream-agent-darwin-arm64)
   - **macOS (x86_64)**: [tailstream-agent-darwin-amd64](https://github.com/tailstream-io/tailstream-agent/releases/latest/download/tailstream-agent-darwin-amd64)
   - **All releases**: [GitHub Releases](https://github.com/tailstream-io/tailstream-agent/releases)
2. **Make executable and run setup** (first time only):
   ```bash
   chmod +x tailstream-agent-linux-amd64
   ./tailstream-agent-linux-amd64
   ```
3. **Enter your Stream ID and Access Token** when prompted
4. **Done!** The agent will automatically discover and stream your logs

For production use, run the agent as a non-root user. The one-liner installer configures this automatically.

### Building from Source

```bash
# Build for Linux
./build-linux.sh

# Or build manually
cd agent
go build -o tailstream-agent
```

### First Run Setup

When you run the agent for the first time, it will automatically launch an interactive setup wizard:

```
🚀 Tailstream Agent Setup
Let's get you set up! This wizard will create a config file for easy future use.

Enter your Tailstream Stream ID: 0199289c-bb03-7275-9529-bee5e9ee1d02
Enter your Tailstream Access Token: your-access-token

✅ Configuration saved to tailstream.yaml
🎉 Setup complete! You can now run the agent without any arguments.
```

After setup, simply run:
```bash
./tailstream-agent-linux-amd64          # Normal operation
./tailstream-agent-linux-amd64 --debug  # With debug output
```

### Service Management (One-Liner Installation)

If you used the one-liner installer, the agent runs as a systemd service:

```bash
# Service status and control
sudo systemctl status tailstream-agent     # Check service status
sudo systemctl stop tailstream-agent       # Stop the service
sudo systemctl start tailstream-agent      # Start the service
sudo systemctl restart tailstream-agent    # Restart the service

# View logs
sudo journalctl -u tailstream-agent -f     # Follow live logs
sudo journalctl -u tailstream-agent        # View all logs

# Configuration
sudo nano /etc/tailstream/agent.yaml       # Edit configuration

# Uninstall
curl https://install.tailstream.io | sudo bash -s -- --uninstall
```

The systemd service is configured to run as the `tailstream` user by default. You do not need root privileges to run or operate the agent after installation.

### Log File Permissions

The installer automatically grants the `tailstream` user access to common log directories:

- **ACL method** (preferred): Precise permissions only for specific log files
- **Group method** (fallback): Adds user to `adm` group for broader log access

Grant access to additional logs:
```bash
sudo setfacl -m u:tailstream:r /path/to/custom.log
```

## 🔄 Automatic Updates

The agent includes built-in automatic updates that are **enabled by default**. This ensures your agent stays current with the latest features and security patches without manual intervention.

### How It Works

- **Background Checks**: Checks for updates every hour via GitHub API
- **Frictionless Self-Updates**: Agent can update itself thanks to `/opt/tailstream` ownership by the `tailstream` user
- **Systemd Integration**: Automatically restarts the service after successful updates
- **Symlink Compatibility**: Updates real binary in `/opt/tailstream/bin`, symlink remains valid
- **Checksum Verification**: Validates downloaded binaries for security

### Configuration

```yaml
updates:
  enabled: true          # Enable auto-updates (default)
  channel: stable        # Update channel: stable, beta, or latest
  check_hours: 1         # Check frequency in hours
```

**Update Channels:**
- **`stable`** (default): Only official stable releases (recommended for production)
- **`beta`**: Includes beta/pre-release versions for early testing
- **`latest`**: Any release including the very latest (development/testing only)

### Manual Update Check

You can manually check for and install updates at any time:

```bash
# Manual update check (works even if auto-updates are disabled)
tailstream-agent update

# With custom config file
tailstream-agent --config /path/to/config.yaml update
```

### System-Wide Installation Updates

The agent is designed to update itself automatically! The installer creates a `/opt/tailstream` directory owned by the `tailstream` user, allowing frictionless self-updates.

**Manual updates** (if needed):
```bash
# Check current status
tailstream-agent status

# Force immediate update check
tailstream-agent update
```

**Emergency manual update** (if auto-update fails):
```bash
curl -fsSL https://install.tailstream.io | sudo bash
```

**Note**: Auto-updates work seamlessly with the default installation. No administrator intervention required!

### Checking Update Status

```bash
# Check if updates are available
tailstream-agent status

# View systemd logs for update notifications
sudo journalctl -u tailstream-agent | grep -i update
```

### Disabling Auto-Updates

To disable automatic updates, set `updates.enabled: false` in your configuration file:

```bash
sudo nano /etc/tailstream/agent.yaml
```

Even with auto-updates disabled, you can still use the manual update command.

### Uninstalling

To completely remove the Tailstream Agent:

```bash
curl -fsSL https://install.tailstream.io | sudo bash -s -- --uninstall
```

This will:
- Stop and disable the systemd service
- Remove all binaries and symlinks
- Remove configuration files
- Remove the `tailstream` user (if no processes running)
- Clean up all installation artifacts

## Configuration

### Configuration File

The agent looks for configuration files in the following order:

1. **System-wide locations** (recommended for production):
   - Linux: `/etc/tailstream/agent.yaml`
   - macOS: `/usr/local/etc/tailstream/agent.yaml`
2. **Current directory**: `tailstream.yaml` (development/testing)

After running the setup wizard, a configuration file is created with your settings:

```yaml
env: production
key: your-access-token
discovery:
  enabled: true
  paths:
    include:
      - "/var/log/nginx/*.log"
      - "/var/log/caddy/*.log"
      - "/var/log/apache2/*.log"
      - "/var/log/httpd/*.log"
    exclude:
      - "**/*.gz"
      - "**/*.1"
updates:
  enabled: true          # Auto-updates enabled by default
  channel: stable        # Update channel: stable, beta, or latest
  check_hours: 1         # Check for updates hourly
ship:
  stream_id: "your-stream-id"
```

### Multi-Stream Configuration

For advanced setups, you can configure multiple Tailstream destinations with different log sources:

```yaml
env: production
key: default-access-token  # Global fallback token

# Multi-stream configuration
streams:
  - name: "nginx-logs"
    stream_id: "stream-id-1"  # URL auto-constructed: https://app.tailstream.io/api/ingest/stream-id-1
    paths:
      - "/var/log/nginx/*.log"
      - "/var/log/nginx/sites/*access.log"
    exclude:
      - "**/*.gz"
      - "**/*.1"
    # Optional: stream-specific access token
    # key: "stream-specific-token"

  - name: "application-logs"
    stream_id: "stream-id-2"
    key: "different-access-token"  # This stream uses its own token
    paths:
      - "/opt/app/logs/*.log"
    exclude:
      - "**/*.old"

  - name: "system-logs"
    stream_id: "stream-id-3"
    # Uses global 'key' since no stream-specific key provided
    paths:
      - "/var/log/syslog*"
      - "/var/log/auth.log"
```

#### Multi-Stream Benefits

- **Separate destinations**: Send different log types to different Tailstream streams
- **Independent access control**: Use different access tokens per stream
- **Organized log routing**: Route nginx logs, application logs, and system logs separately
- **Flexible patterns**: Each stream can have its own include/exclude patterns
- **Format agnostic**: All logs are sent as raw text - backend handles parsing
- **Mixed formats per stream**: A single stream can handle multiple file formats simultaneously
- **Backward compatible**: Existing single-stream configs continue to work

### Manual Configuration

If you prefer not to use the setup wizard, you can configure the agent manually:

#### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TAILSTREAM_KEY` | - | Your Tailstream access token |
| `TAILSTREAM_ENV` | `production` | Environment label |

#### Command Line Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to YAML configuration file (default: system-wide or tailstream.yaml) |
| `--env` | Override environment label |
| `--debug` | Enable verbose debug output |

#### Configuration Options

**Discovery Settings:**

- `discovery.enabled` (bool): Enable/disable automatic log file discovery
- `discovery.paths.include` ([]string): Glob patterns for log files to monitor
- `discovery.paths.exclude` ([]string): Glob patterns for files to ignore

**Default Include Patterns:**
- `/var/log/nginx/*.log` - Nginx access/error logs
- `/var/log/caddy/*.log` - Caddy web server logs
- `/var/log/apache2/*.log` - Apache logs
- `/var/log/httpd/*.log` - Apache/httpd logs

**Default Exclude Patterns:**
- `**/*.gz` - Compressed log files
- `**/*.1` - Rotated log files

**Auto-Update Settings:**

- `updates.enabled` (bool): Enable automatic updates (default: true)
- `updates.channel` (string): Update channel - `stable` (default), `beta`, or `latest`
- `updates.check_hours` (int): Hours between update checks (default: 1)

**Shipping Settings:**

- `ship.stream_id` (string): Tailstream stream ID (URL auto-constructed as https://app.tailstream.io/api/ingest/{stream_id})

### Usage Examples

#### Recommended - Setup wizard (first time):
```bash
./tailstream-agent-linux-amd64
# Follow the interactive prompts
```

#### Subsequent runs:
```bash
./tailstream-agent-linux-amd64          # Normal operation
./tailstream-agent-linux-amd64 --debug  # With debug output
```

#### Manual update check:
```bash
./tailstream-agent-linux-amd64 update   # Check for updates manually
./tailstream-agent-linux-amd64 version  # Show current version
./tailstream-agent-linux-amd64 help     # Show help
```

#### Manual configuration with YAML:
```bash
# Create tailstream.yaml
cat > tailstream.yaml << EOF
env: production
key: your-access-token
ship:
  stream_id: your-stream-id
discovery:
  enabled: true
  paths:
    include:
      - "/var/log/nginx/*.log"
      - "/var/log/caddy/*.log"
EOF

./tailstream-agent-linux-amd64
```

#### Using custom configuration file:
```bash
./tailstream-agent-linux-amd64 --config /etc/tailstream/agent.yaml
```

#### Docker usage (with config file):
```bash
# Create config file first
cat > tailstream.yaml << EOF
env: production
key: your-access-token
ship:
  stream_id: your-stream-id
discovery:
  enabled: true
  paths:
    include:
      - "/var/log/nginx/*.log"
      - "/var/log/caddy/*.log"
EOF

# Run with mounted config
docker run -v $(pwd)/tailstream.yaml:/tailstream.yaml \
  -v /var/log:/var/log:ro \
  tailstream-agent
```

#### Docker usage (environment variables):
```bash
docker run -e TAILSTREAM_KEY=your-token \
  -v /var/log:/var/log:ro \
  tailstream-agent
```

Note: With environment variables only, you'll need to run the setup wizard on first launch to configure the stream ID.

#### Stdin mode (pipe any log source):
Perfect for ad-hoc debugging, Kubernetes, Docker, or any streaming log source:

```bash
# First, securely store your access token (one time):
echo 'your-access-token' > ~/.tailstream-key && chmod 600 ~/.tailstream-key

# Then pipe logs from any source:
tail -f /var/log/nginx/access.log | tailstream-agent --stream-id <stream-id> --key-file ~/.tailstream-key
kubectl logs -f pod-name | tailstream-agent --stream-id <stream-id> --key-file ~/.tailstream-key
docker logs -f container-name | tailstream-agent --stream-id <stream-id> --key-file ~/.tailstream-key
journalctl -f | tailstream-agent --stream-id <stream-id> --key-file ~/.tailstream-key

# Or use any custom command:
./my-app --verbose 2>&1 | tailstream-agent --stream-id <stream-id> --key-file ~/.tailstream-key
```

**Stdin mode features:**
- 🔒 **Secure** - Key stored in file with `chmod 600`, never exposed in process listings or shell history
- 🚀 **Zero configuration** - No config file needed, just `--stream-id` and `--key-file`
- 📦 **Portable** - Single binary, works anywhere Go runs
- 💨 **Low latency** - Ships batches every 100 events or 2 seconds

## How It Works

1. **Discovery**: The agent scans filesystem paths using glob patterns to find log files
2. **Tailing**: Continuously monitors discovered files for new lines (similar to `tail -f`)
3. **Batching**: Collects up to 100 events or waits 2 seconds before shipping
4. **Shipping**: Sends raw log lines via HTTP POST to Tailstream ingest API as NDJSON

### Log Handling

The agent streams all log lines as raw text to the Tailstream backend:

- **No local parsing** - Logs are sent exactly as written
- **Format agnostic** - Works with any log format (nginx, apache, JSON, syslog, custom formats, etc.)
- **Backend processing** - The Tailstream backend handles all parsing and format detection
- **Simple and reliable** - What you write is what gets shipped

## Testing

### Running Tests

```bash
cd agent
go test ./...
```

### Docker Installation Test

Verify the Docker build works correctly:

```bash
./agent/docker-install-test.sh
```

This builds the container and runs it with `-h` to verify the binary starts.

## Troubleshooting

### No logs being shipped

1. Check that log files exist and match your include patterns
2. Verify the agent has read permissions on log files
3. Ensure your `TAILSTREAM_KEY` is correct
4. Check agent logs for discovery and shipping errors

### Permission issues

The agent needs read access to log files. When running in Docker, ensure proper volume mounts and consider running with appropriate user permissions.

### Custom log locations

Add custom paths to your configuration file's `discovery.paths.include` section using glob patterns.

## Development

Built with Go 1.22+ using:
- `github.com/bmatcuk/doublestar/v4` for glob pattern matching
- `gopkg.in/yaml.v3` for YAML configuration parsing

## License

MIT License
