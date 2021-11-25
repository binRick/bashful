#!/usr/bin/env bash
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
set -e
default_watch_file="pkg -w example"
watch_file="${1:-$default_watch_file}"
shift || true
cmd="./${@:-./build.sh}"
#cmd="nodemon -I --delay .4 -w pkg -w example -e sh,go,yaml,yml -x sh -- -c '$cmd||true'"
cmd="nodemon -I --delay .1 -w $watch_file -x sh -- -c './${cmd}||true'"
eval "$cmd"
