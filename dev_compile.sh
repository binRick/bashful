#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

cmd="nodemon -w . -e sh,go,yml -I -x sh -- -c 'reset && ./compile.sh && passh ./test_forks.sh $@ && killall bashful||killall -9 bashful||true'"

exec $cmd
