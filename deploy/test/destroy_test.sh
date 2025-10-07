#!/usr/bin/env bash
set -euo pipefail

STACK_NAME="koorde-test-stack"

echo "[INFO] Deleting CloudFormation stack '$STACK_NAME'..."
aws cloudformation delete-stack --stack-name "$STACK_NAME"

echo "[INFO] Waiting for deletion to complete..."
aws cloudformation wait stack-delete-complete --stack-name "$STACK_NAME"

echo "[SUCCESS] Stack '$STACK_NAME' deleted successfully."
