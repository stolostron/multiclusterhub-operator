#!/bin/bash

RESULTS_PATH=$1
RES=$(cat $RESULTS_PATH)
SLACK_MESSAGE=":sadblob: Tests Failed! :sadpuppy:"
NOFAIL='failures="0"'
if [[ $RES == *$NOFAIL* ]]; 
then
    SLACK_MESSAGE=":happy-hulk: Tests Passed! :success-kid:"
fi
DATE=$(date)
SLACK_MESSAGE="$GITHUB_WORKFLOW : $DATE\n$SLACK_MESSAGE"
echo $SLACK_MESSAGE
curl -X POST -H 'Content-type: application/json' --data "{'text':'${SLACK_MESSAGE}'}" ${SLACKBOT_WEBHOOK_URL}