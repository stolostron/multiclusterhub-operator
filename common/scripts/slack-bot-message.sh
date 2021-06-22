#!/bin/bash

RESULTS_PATH=$1
RES=$(cat $RESULTS_PATH)
SLACK_MESSAGE="TESTS FAILED! :("
NOFAIL='failures="0"'
if [[ $RES == *$NOFAIL* ]]; 
then
    SLACK_MESSAGE="TESTS PASSED! :)"
fi
echo $SLACK_MESSAGE
curl -X POST -H 'Content-type: application/json' --data "{'text':'${SLACK_MESSAGE}'}" ${SLACKBOT_WEBHOOK_URL}