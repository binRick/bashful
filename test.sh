#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
f="${1:-example/00-demo.yml}"
shift || true
a="${@:---only-tags t}"
go build -o ./bf .
err=$(mktemp)
dorun() {
	(
		reap ./bf run $f $a ||
			reap ./bf run example/05-minimal.yml
	)
}
dorun 2>.ee || cat .ee
#| tee $err | tee .ee || cat $err

unlink $err

exit

set +e
extrace -Ql -o .e passh ./bf run $f $a

if [[ -f .e ]]; then
	echo -ne "forks: "
	wc -l .e
	echo -ne "sttys: "
	grep -c stty .e

	trap "unlink .e" EXIT

fi
