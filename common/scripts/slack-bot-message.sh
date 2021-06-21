#!/bin/bash
SLACK_MESSAGE=$1
curl -X POST -H 'Content-type: application/json' --data "{'text':'${SLACK_MESSAGE}'}" ${SLACKBOT_WEBHOOK_URL}