#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

go build -o bf .
[[ -f bashful ]] && unlink bashful || true
cp -prvf bf bashful
[[ -f dist/bashful ]] && unlink dist/bashful || true
[[ -d dist ]] && cp -prvf bashful dist/bashful

true
