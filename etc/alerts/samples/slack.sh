#!/bin/bash

# Alert parameters are stored in environment:
# TYPE ("GOOD" / "BAD")
# SUBJECT
# CONTENT

hook_url="https://hooks.slack.com/services/xxx"

mark=":exclamation:"
if [ "$TYPE" = "GOOD" ]; then
    mark=":heavy_check_mark:"
fi

jq --version > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "You need the 'jq' utility to send alerts"
    exit 1
fi

payload=$(
    jq -n \
    --arg text "$mark [$TYPE] $(hostname -s) : *$SUBJECT* - $CONTENT" \
    '{text: $text}' \
)

curl -s -f -w "\nHTTP Code %{http_code}\n" -X POST --data-urlencode "payload=$payload" "$hook_url"
if [ $? -ne 0 ]; then
    exit 1
fi
