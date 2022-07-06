#!/usr/bin/env python3
# Copyright (c) 2021 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project
# Assumes: Python 3.6+

import yaml
import logging


def validateFieldMapping(csv, ruleType, fieldMap):
    errs = []
    fields = fieldMap.split('.') # Split the field map into a list of fields
    for x, field in enumerate(fields):
        if x == len(fields) - 1: # If this is the last field in the list, we're at the end of the path
            
            if ruleType == 'disallowedFields': # Logic if key is disallowed
                if field not in csv:
                    continue # Field does not exist, so we can skip it
                if csv[field] == {}:
                    csv.pop(field) # Field is empty, remove it
                else:
                    errs.append("DISALLOWED: '{}'".format(fieldMap))

            elif ruleType == 'requiredFields': # Logic if key is required
                if field not in csv or csv[field] == {}:
                    errs.append("REQUIRED: '{}'".format(fieldMap)) # If field does not exist or is empty, return error
                else:
                    csv.pop(field) # Field exists and is not empty, remove it

            elif ruleType == 'noOpFields' or ruleType == 'optionalFields': # Logic if key is no-op or optional
                if field in csv: # If field exists, remove it
                    csv.pop(field)

            else:
                errs.append("Unknown rule type: {}".format(ruleType)) # Unknown rule type, return error, likely fatal
                return errs    
        else:
            csv = csv[field] # Continue looping through the path
    return errs


def validateCSV(csvPath):
    errs = []
    # Load the CSV Linter rules
    with open("hack/bundle-automation/csv_linter_rules.yaml", 'r') as f:
        rules = yaml.safe_load(f)

    with open(csvPath, 'r') as f:
        csv = yaml.safe_load(f)
    
    name = csv['metadata']['name']

    logging.info("Linting CSV: %s", name)
    for fieldMap in rules['disallowedFields']:
        errs.extend(validateFieldMapping(csv, 'disallowedFields', fieldMap))
    for fieldMap in rules['requiredFields']:
        errs.extend(validateFieldMapping(csv, 'requiredFields', fieldMap))
    for fieldMap in rules['noOpFields']:
        errs.extend(validateFieldMapping(csv, 'noOpFields', fieldMap))
    for fieldMap in rules['optionalFields']:
        errs.extend(validateFieldMapping(csv, 'optionalFields', fieldMap))

    # These subfields should be empty after the above rules are applied. Treat as disallowed to remove them if they are empty.
    errs.extend(validateFieldMapping(csv, 'disallowedFields', "spec.install.spec"))
    errs.extend(validateFieldMapping(csv, 'disallowedFields', "spec.install"))
    errs.extend(validateFieldMapping(csv, 'disallowedFields', "spec"))
    csv.pop('metadata') # Remove metadata from the CSV
    
    if csv != {}:
        errs.append(name + " CSV is not empty")

    return errs