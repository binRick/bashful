#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

cmd="nodemon --signal SIGINT -w . -e sh,go,yml -I -x sh -- -c 'reset && passh ./test_forks.sh $@||true'"

exec $cmd
