#!/bin/bash
set -e
extrace -Ql -o .dd.e passh ./bf run example/dd.yml

if [[ -f .e ]]; then
	echo -ne "forks: "
	wc -l .dd.e
	echo -ne "sttys: "
	grep -c stty .dd.e

	trap "unlink .e" EXIT

fi
