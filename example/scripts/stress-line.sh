#!/usr/bin/env bash
# set -u
for i in $(seq 1 $1) ;do
    echo "Stress tester $i/$1: $(date)" .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM  
done