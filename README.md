# Tailstream Agent

A lightweight Go agent that automatically discovers and parses common web server access logs, converting them to structured JSON format and shipping them to the Tailstream ingest API for real-time log[...]

## ⚡ Quick Start

### One-Liner Installation (Recommended)

Install and set up the agent as a system service with a single command:

```bash
curl -fsSL https://install.tailstream.io | sudo bash
```

This will:
- Download the correct binary for your Linux architecture (x86_64/ARM64)
- Install to `/usr/local/bin/tailstream-agent`
- Create a dedicated `tailstream` user
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

[...]

## Troubleshooting

### Permission issues

The agent needs read access to log files. When running in Docker, ensure proper volume mounts and consider running with appropriate user permissions.
You do not need to run the agent as root; just ensure the `tailstream` user (or your chosen non-root user) has read access.

[... rest unchanged ...]
