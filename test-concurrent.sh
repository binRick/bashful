#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
export TIMEHISTORY_ENABLED=1
#LF='/tmp/c0*-concurrent-stats.json'
#[[ -f "$LF" ]] && unlink $LF
./bashful run example/00-concurrent.yml ${@:-}

#eval "cat $LF" |jq  '.[].args' -rc|egrep -v 'tmp/concurrent|base64|bashful/.logs|%F@%T'
