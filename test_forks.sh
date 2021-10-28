#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
f="${1:-example/00-demo.yml}"
shift || true
a="${@:---only-tags t}"
go build -o ./bf .
err=$(mktemp)

pfx="extrace -Ql -o .e passh reap"
cmd_run="./bf run"
cmd_run="$pfx $cmd_run"

cc() {
	if [[ -f .e ]]; then
		echo -ne "forks: "
		wc -l .e
		echo -ne "sttys: "
		grep -c stty .e

		trap "unlink .e" EXIT

	fi
	if [[ "$OK" != 1 ]]; then
    reset
  fi
}
dorun() {
	(
		eval $cmd_run $f $a || {			eval $cmd_run example/05-minimal.yml; exit 1; }
    eval $cmd_run example/dd.yml
	)
}
trap cc EXIT
dorun 2>.ee || cat .ee
unlink $err
echo
export OK=1
