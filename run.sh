#!/usr/bin/bash
# --norc --noprofile
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

go run . $@
