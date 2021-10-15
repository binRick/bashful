#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

set -x
go build -o bashful .
rsync bashful bf
rsync bashful dist/.
