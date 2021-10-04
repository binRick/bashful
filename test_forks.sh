#!/bin/bash
set -e 
DEMO_FILE=${1:-example/00-demo.yml}
go build -o ./bf .
set +e
extrace -Ql -o .e passh ./bf run $DEMO_FILE


echo -ne "forks: "; wc -l .e

echo -ne "sttys: "; grep -c stty .e

trap "unlink .e" EXIT
