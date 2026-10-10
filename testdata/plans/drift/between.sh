#!/usr/bin/env bash
# Runs after apply and before plan, against the moto endpoint: a human changes the
# queue timeout in the console and deletes the parameter.
set -euo pipefail
url=$(aws sqs get-queue-url --queue-name fixture-drift-events --query QueueUrl --output text)
aws sqs set-queue-attributes --queue-url "$url" --attributes VisibilityTimeout=120
aws ssm delete-parameter --name /fixture/drift/flag
