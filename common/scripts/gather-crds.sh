#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Tested on Mac only


rm -rf common/chart-temp
rm -rf chartCRDs/
mkdir -p chartCRDs/

# Loop through charts in common/config/chartSHA.csv and clone
while IFS=, read -r gitURL branch
do
    mkdir -p common/chart-temp
    git clone $gitURL --branch $branch common/chart-temp
    cd common/chart-temp

    for crd in $(find . | xargs grep CustomResourceDefinition 2> /dev/null | cut -d ':' -f1)
    do
        echo $crd
        cp $crd ../../chartCRDs/
    done

    cd ../..
    rm -rf common/chart-temp
done < common/config/chartSHA.csv
