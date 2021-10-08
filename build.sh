#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

./compile.sh
rsync bashful ~/.local/bin/bashful
rsync bashful $(which bashful)
[[ -d ~/vpntech-haproxy-container/files ]] && rsync bashful ~/vpntech-haproxy-container/files/bashful
[[ -d /opt/vpntech-binaries/x86_64 ]] && rsync bashful /opt/vpntech-binaries/x86_64/bashful
