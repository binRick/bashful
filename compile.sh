#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

go build -o bf .
cp -prvf bf bashful
[[ -d dist ]] && cp -prvf bashful dist/bashful
