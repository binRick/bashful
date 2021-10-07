#!/bin/bash
set -e 
go build -o ./bf .
set +e
extrace -Ql -o .e passh ./bf run $@


echo -ne "forks: "; wc -l .e

echo -ne "sttys: "; grep -c stty .e

trap "unlink .e" EXIT
