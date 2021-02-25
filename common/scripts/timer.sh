# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

#!/bin/bash

make cr
THEN=$(date +%s)
while :
do
    if [[ "$(oc get mch)" == *"Running"* ]]; then
        echo "RUNNING!"
        break
    fi
    echo "$(oc get mch | tail -n 1 | awk '{ print $3 }')"
    sleep 1
done
NOW=$(date +%s)
SECONDS=$(expr $NOW - $THEN)
echo "$(expr $SECONDS / 60) minutes $(expr $SECONDS % 60) seconds"
