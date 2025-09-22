# Tailstream Agent

A lightweight Go agent that automatically discovers common web server logs, normalizes them into JSON format, and ships batches of records to the Tailstream ingest API for real-time log analysis.

## Quick Start

### Prerequisites

- Go 1.22+ (for building from source)
- A Tailstream API key (`ts_live_xxx` or `ts_test_xxx`)

### Installation

#### Building from Source

```bash
cd agent
go build -o tailstream-agent
```

#### Docker

Build the Docker image:

```bash
docker build -t tailstream-agent ./agent
```

### Basic Usage

Set your Tailstream API key and run the agent:

```bash
export TAILSTREAM_KEY=ts_live_your_api_key_here
./tailstream-agent
```

The agent will automatically discover and tail common web server log files.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TAILSTREAM_KEY` | - | **Required.** Your Tailstream API key |
| `TAILSTREAM_ENV` | `production` | Environment label added to all log events |
| `TAILSTREAM_URL` | `https://ingest.tailstream.com/v1/batch` | Tailstream ingest endpoint URL |

### Command Line Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to YAML configuration file |
| `--env` | Override environment label |
| `--key-file` | Path to file containing API key |
| `--ship-url` | Override ingest endpoint URL |

### Configuration File

Create a YAML configuration file to customize agent behavior:

```yaml
env: production

discovery:
  enabled: true
  paths:
    include:
      - "/var/log/nginx/*.log"
      - "/var/log/caddy/*.log"
      - "/var/log/apache2/*.log"
      - "/var/log/httpd/*.log"
      - "/var/www/**/storage/logs/*.log"
      - "/custom/path/*.log"
    exclude:
      - "**/*.gz"
      - "**/*.1"
      - "**/*.old"

ship:
  url: "https://ingest.tailstream.com/v1/batch"
```

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
- `/var/www/**/storage/logs/*.log` - Laravel/PHP application logs

**Default Exclude Patterns:**
- `**/*.gz` - Compressed log files
- `**/*.1` - Rotated log files

**Shipping Settings:**

- `ship.url` (string): Tailstream ingest endpoint URL

### Usage Examples

#### Basic usage with environment variable:
```bash
TAILSTREAM_KEY=ts_live_xxx ./tailstream-agent
```

#### With custom configuration file:
```bash
TAILSTREAM_KEY=ts_live_xxx ./tailstream-agent --config /etc/tailstream/agent.yaml
```

#### Using key file and custom environment:
```bash
./tailstream-agent --key-file /etc/tailstream/key.txt --env staging
```

#### Docker usage:
```bash
docker run -e TAILSTREAM_KEY=ts_live_xxx \
  -v /var/log:/var/log:ro \
  tailstream-agent
```

#### Docker with custom config:
```bash
docker run -e TAILSTREAM_KEY=ts_live_xxx \
  -v /var/log:/var/log:ro \
  -v /path/to/config.yaml:/config.yaml \
  tailstream-agent --config /config.yaml
```

## How It Works

1. **Discovery**: The agent scans filesystem paths using glob patterns to find log files
2. **Tailing**: Continuously monitors discovered files for new lines (similar to `tail -f`)
3. **Parsing**: Attempts to parse each line as JSON; falls back to raw text if parsing fails
4. **Enrichment**: Adds metadata (environment, hostname, source file) to each log entry
5. **Batching**: Collects up to 100 events or waits 2 seconds before shipping
6. **Shipping**: Sends batches via HTTP POST to Tailstream ingest API

### Log Format Support

- **JSON logs**: Parsed and forwarded with original structure intact
- **Plain text logs**: Wrapped in JSON with the raw line in a `line` field

All logs are enriched with:
- `env`: Environment label
- `host`: Hostname where agent is running
- `file`: Source log file path

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

[Add your license information here]