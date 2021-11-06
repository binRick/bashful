#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

./compile.sh
if command -v rsync; then
  if [[ -d ~/.local/bin ]]; then
  	rsync bashful ~/.local/bin/bashful
  fi
  if uname -s |grep -i darwin; then
    echo darwin
  else
  if command -v bashful; then
  	rsync bashful $(command -v bashful)
  fi
	rsync bashful /usr/bin/bashful||true
	[[ -d ~/vpntech-haproxy-container/files ]] && rsync bashful ~/vpntech-haproxy-container/files/bashful
	[[ -d /opt/vpntech-binaries/x86_64 ]] && rsync bashful /opt/vpntech-binaries/x86_64/bashful
  fi
else
	cp bashful /usr/bin/bashful
fi

true
