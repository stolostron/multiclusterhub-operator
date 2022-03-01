#!/usr/bin/env bash

########################
# include the magic
########################
. ./demo-magic.sh


########################
# Configure the options
########################

#
# speed at which to simulate typing. bigger num = faster
#
# TYPE_SPEED=20

#
# custom prompt
#
# see http://www.tldp.org/HOWTO/Bash-Prompt-HOWTO/bash-prompt-escape-sequences.html for escape sequences
#
DEMO_PROMPT="${GREEN}➜ ${CYAN}\W "

# text color
# DEMO_CMD_COLOR=$BLACK

# hide the evidence
clear


pe "oc get mch multiclusterhub -oyaml"
pe "oc get project cluster-backup"
pe "oc patch --type=merge mch multiclusterhub -p '{\"spec\":{\"enableClusterBackup\":true}}'"
pe "oc project cluster-backup"
pe "oc get subscription"
pe "oc project open-cluster-management"
pe "oc patch --type=merge mch multiclusterhub -p '{\"spec\":{\"enableClusterBackup\":false}}'"
pe "oc project cluster-backup"
# show a prompt so as not to reveal our true nature after
# the demo has concluded
p ""