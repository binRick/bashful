#!/bin/bash
set -ex
go build -o bashful .
rsync bashful ~/.local/bin/bashful
rsync bashful $(which bashful)
rsync bashful ~/vpntech-haproxy-container/files/bashful
