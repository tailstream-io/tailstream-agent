#!/usr/bin/env bash
set -euo pipefail
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
docker build -t tailstream-agent-test "$DIR"
docker run --rm tailstream-agent-test -h >/dev/null
