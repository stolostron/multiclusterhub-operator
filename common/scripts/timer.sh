#!/bin/bash

make cr
THEN=$(date +%s)
while :
do
    if [[ "$(oc get mch)" == *"Running"* ]]; then
        echo "RUNNING!"
        break
    fi
    echo "NOT YET..."
    sleep 1
done
NOW=$(date +%s)
SECONDS=$(expr $NOW - $THEN)
echo "$(expr $SECONDS / 60) minutes $(expr $SECONDS % 60) seconds"