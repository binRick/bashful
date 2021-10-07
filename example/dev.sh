#!/bin/bash
set -ex
ls -altr /proc/self/fd
echo -e "ARGS=$@"
env|egrep '^__'||true
#date > &4
