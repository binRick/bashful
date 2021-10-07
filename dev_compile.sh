#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

cmd="nodemon -w . -e sh,go,yml -I -x sh -- -c './compile.sh && passh ./test_forks.sh $@||true'"

exec $cmd
