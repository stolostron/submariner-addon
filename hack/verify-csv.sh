#!/bin/bash

# Verify regenerating OLM CSV doesn't result in changes
if [ ! git diff --exit-code deploy/olm-catalog ] ; then
    echo "There are already changes to the CSV, can't verify regeneration doesn't cause changes."
    exit 1
fi
make update-csv
if [ ! git diff --exit-code deploy/olm-catalog ] ; then
    echo "Regenerating CSV (make update-csv ) resulted in changes."
    echo "Commit the CSV updates along with the changes that cause them."
    exit 1
fi
