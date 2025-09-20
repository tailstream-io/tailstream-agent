# Tailstream Agent

The Tailstream agent is a minimal Go program that autodiscovers common web
server logs, normalizes them, and ships batches of JSON records to the
Tailstream ingest API.

## Building from Source

```bash
cd agent
go build -o tailstream-agent
```

Run the agent by providing your Tailstream API key and optional configuration
file:

```bash
TAILSTREAM_KEY=ts_live_xxx ./tailstream-agent --config /etc/tailstream/agent.yaml
```

## Docker Image

A Dockerfile is provided for convenience. Build and test the container with:

```bash
./agent/docker-install-test.sh
```

This builds a minimal image and runs the agent with the `-h` flag to verify the
binary starts correctly.

## Testing Locally

Unit and integration tests cover discovery, parsing, and end-to-end shipping.
Run them with:

```bash
go test ./agent/...
```

The integration test compiles the binary, tails a temporary log file, and
verifies the record is delivered to a stub HTTP server.
