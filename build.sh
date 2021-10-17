#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

./compile.sh
if command -v rsync; then
rsync bashful ~/.local/bin/bashful
rsync bashful $(which bashful)
rsync bashful /usr/bin/bashful
[[ -d ~/vpntech-haproxy-container/files ]] && rsync bashful ~/vpntech-haproxy-container/files/bashful
[[ -d /opt/vpntech-binaries/x86_64 ]] && rsync bashful /opt/vpntech-binaries/x86_64/bashful
else
	cp bashful /usr/bin/bashful
fi

true
