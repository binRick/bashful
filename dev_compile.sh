#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
EXEC_ARGS="${@:-echo OK}"
cmd="nodemon -w . -e sh,go,yml,mod,go,j2,txt --signal SIGKILL -x env -- bash -c 'reset && ./compile.sh||true; $EXEC_ARGS'"

git_pull() {
	(
		set +e
		while :; do
			git pull || git pull
			sleep 15
		done
	) &

}

# && passh ./test_forks.sh $@ && { set +e; killall bashful 2>/dev/null||killall -9 bashful 2>/dev/null; killall bf 2>/dev/null; }'"

git_pull
eval $cmd
