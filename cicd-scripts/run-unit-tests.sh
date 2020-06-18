#!/bin/bash -e
# Copyright (c) 2020 Red Hat, Inc.

_script_dir=$(dirname "$0")
mkdir -p test/coverage
echo 'mode: atomic' > test/coverage/cover.out
echo '' > test/coverage/cover.tmp
echo -e "${GOPACKAGES// /\\n}" | xargs -n1 -I{} $_script_dir/test-package.sh {} ${GOPACKAGES// /,}

if [ ! -f test/coverage/cover.out ]; then
    echo "Coverage file test/coverage/cover.out does not exist"
    exit 0
fi

COVERAGE=$(go tool cover -func=test/coverage/cover.out | grep "total:" | awk '{ print $3 }' | sed 's/[][()><%]/ /g')
echo "-------------------------------------------------------------------------"
echo "TOTAL COVERAGE IS ${COVERAGE}%"
echo "-------------------------------------------------------------------------"

go tool cover -html=test/coverage/cover.out -o=test/coverage/cover.html
