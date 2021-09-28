#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

go build -o bashful .
rsync bashful ~/.local/bin/bashful
rsync bashful $(which bashful)
rsync bashful ~/vpntech-haproxy-container/files/bashful
