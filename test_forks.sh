#!/bin/bash
set -e 
f="${1:-example/00-demo.yml}"; shift||true
a="${@:---only-tags t}"
go build -o ./bf .
./bf run example/05-minimal.yml --only-tags t00 2>.ee
set +e
extrace -Ql -o .e passh ./bf run $f $a



if [[ -f .e ]]; then 
echo -ne "forks: "; wc -l .e;  echo -ne "sttys: "; grep -c stty .e; 

trap "unlink .e" EXIT

fi
