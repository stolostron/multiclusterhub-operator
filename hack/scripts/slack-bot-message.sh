# Copyright Contributors to the Open Cluster Management project

#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

RESULTS_PATH=$1
RES=$(cat $RESULTS_PATH)
SLACK_MESSAGE=":sadblob: Tests Failed! :sadpuppy:"
NOFAIL='failures="0"'
if [[ $RES == *$NOFAIL* ]]; 
then
    SLACK_MESSAGE=":happy-hulk: Tests Passed! :success-kid:"
fi
DATE=$(date)
BUILD_URL="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
SLACK_MESSAGE="$GITHUB_WORKFLOW : $DATE\n$SLACK_MESSAGE\n$BUILD_URL"
echo $SLACK_MESSAGE
curl -X POST -H 'Content-type: application/json' --data "{'text':'${SLACK_MESSAGE}'}" ${SLACKBOT_WEBHOOK_URL}