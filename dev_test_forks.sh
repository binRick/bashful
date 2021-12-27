#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

cmd="nodemon --signal SIGINT -w bashful -I -x sh -- -c 'reset && passh ./test_forks.sh $@||true'"

exec $cmd
