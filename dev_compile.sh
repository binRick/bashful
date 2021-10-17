#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

cmd="nodemon -w . -e sh,go,yml,mod,go,j2,txt -I --signal SIGKILL -x sh -- -c 'reset && ./compile.sh'"
# && passh ./test_forks.sh $@ && { set +e; killall bashful 2>/dev/null||killall -9 bashful 2>/dev/null; killall bf 2>/dev/null; }'"

exec $cmd
