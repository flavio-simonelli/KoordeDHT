#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Koorde DHT Cluster Destroyer
# -----------------------------------------------------------------------------
# Deletes all CloudFormation stacks created by the Koorde deploy script.
# Stack names are assumed to follow the pattern: koorde-instance-<N>
# -----------------------------------------------------------------------------

usage() {
  echo "Usage: $0 [--region <REGION>]"
  echo
  echo "Deletes all CloudFormation stacks whose names start with 'koorde-instance-'."
  echo "Options:"
  echo "  --region <REGION>  AWS region (default: us-east-1)"
  exit 1
}

# Default region
AWS_REGION="us-east-1"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --region) AWS_REGION="$2"; shift 2 ;;
    *) usage ;;
  esac
done

echo "[INFO] Using AWS region: $AWS_REGION"

# Fetch all stacks with the 'koorde-instance-' prefix
STACK_NAMES=$(aws cloudformation describe-stacks \
  --region "$AWS_REGION" \
  --query "Stacks[].StackName" \
  --output text | tr '\t' '\n' | grep '^koorde-instance-' || true)

if [[ -z "$STACK_NAMES" ]]; then
  echo "[INFO] No stacks found with prefix 'koorde-instance-' in region $AWS_REGION"
  exit 0
fi

echo "[INFO] Found the following stacks to delete:"
echo "$STACK_NAMES"
echo

# Delete each stack and wait for completion
for STACK_NAME in $STACK_NAMES; do
  echo "[INFO] Deleting stack: $STACK_NAME ..."
  aws cloudformation delete-stack \
    --region "$AWS_REGION" \
    --stack-name "$STACK_NAME" || {
      echo "[WARN] Failed to start deletion for $STACK_NAME (skipping)"
      continue
    }

  echo "[INFO] Waiting for $STACK_NAME to be deleted..."
  aws cloudformation wait stack-delete-complete \
    --region "$AWS_REGION" \
    --stack-name "$STACK_NAME" && \
    echo "[SUCCESS] Deleted: $STACK_NAME" || \
    echo "[ERROR] Timeout or failure deleting $STACK_NAME"
done

echo
echo "[DONE] All 'koorde-instance-*' stacks have been processed."
