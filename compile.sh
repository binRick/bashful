#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

go build -o bf .
rsync bf bashful
[[ -d dist ]] && rsync bashful dist/.
