#!/bin/sh

for i in $(seq 1 5); do
    echo "$i: $(date), generated log line"
    sleep 0.01 > /dev/null
done
