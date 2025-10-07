#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# KoordeDHT CloudFormation Launcher (fixed stack name, all params required)
# -----------------------------------------------------------------------------
# Usage example:
#   ./launch_test.sh \
#       --keypair my-key \
#       --s3-bucket koorde-dht \
#       --s3-prefix experiments/test1 \
#       --vpc-id vpc-0abcd1234 \
#       --subnet-id subnet-0abcd5678 \
#       --instance-type t3.medium \
#       --sim-duration 60s \
#       --query-rate 0.5 \
#       --parallel-min 1 \
#       --parallel-max 5 \
#       --delay 200ms \
#       --jitter 50ms \
#       --loss 0.1% \
#       --churn-interval 20 \
#       --churn-min-active 3 \
#       --churn-pjoin 0.4 \
#       --churn-pleave 0.3 \
#       --max-nodes 5
# -----------------------------------------------------------------------------

TEMPLATE_FILE="test_koorde.yml"
STACK_NAME="koorde-test-stack"

# --- Parameters --------------------------------------------------------------
KEYPAIR=""
S3_BUCKET=""
S3_PREFIX=""
VPC_ID=""
SUBNET_ID=""
INSTANCE_TYPE=""
SIM_DURATION=""
QUERY_RATE=""
PARALLEL_MIN=""
PARALLEL_MAX=""
DELAY=""
JITTER=""
LOSS=""
CHURN_INTERVAL=""
CHURN_MIN_ACTIVE=""
CHURN_PJOIN=""
CHURN_PLEAVE=""
MAX_NODES=""

# --- Usage -------------------------------------------------------------------
usage() {
  echo
  echo "Usage:"
  echo "  $0 --keypair <key> --s3-bucket <bucket> --s3-prefix <prefix> \\"
  echo "     --vpc-id <vpc-id> --subnet-id <subnet-id> \\"
  echo "     --instance-type <type> --sim-duration <time> --query-rate <rate> \\"
  echo "     --parallel-min <n> --parallel-max <n> --delay <Xms> --jitter <Yms> --loss <Z%> \\"
  echo "     --churn-interval <sec> --churn-min-active <n> --churn-pjoin <p> --churn-pleave <p> --max-nodes <n>"
  echo
  echo "Example:"
  echo "  ./launch_test.sh --keypair my-key --s3-bucket koorde-dht --s3-prefix experiments/test1 \\"
  echo "     --vpc-id vpc-0abcd1234 --subnet-id subnet-0abcd5678 \\"
  echo "     --instance-type t3.medium --sim-duration 90s --query-rate 0.5 --parallel-min 1 --parallel-max 5 \\"
  echo "     --delay 200ms --jitter 50ms --loss 0.1% --churn-interval 20 --churn-min-active 3 \\"
  echo "     --churn-pjoin 0.4 --churn-pleave 0.3 --max-nodes 5"
  echo
  exit 1
}

# --- Parse Arguments ---------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --keypair) KEYPAIR="$2"; shift 2 ;;
    --s3-bucket) S3_BUCKET="$2"; shift 2 ;;
    --s3-prefix) S3_PREFIX="$2"; shift 2 ;;
    --vpc-id) VPC_ID="$2"; shift 2 ;;
    --subnet-id) SUBNET_ID="$2"; shift 2 ;;
    --instance-type) INSTANCE_TYPE="$2"; shift 2 ;;
    --sim-duration) SIM_DURATION="$2"; shift 2 ;;
    --query-rate) QUERY_RATE="$2"; shift 2 ;;
    --parallel-min) PARALLEL_MIN="$2"; shift 2 ;;
    --parallel-max) PARALLEL_MAX="$2"; shift 2 ;;
    --delay) DELAY="$2"; shift 2 ;;
    --jitter) JITTER="$2"; shift 2 ;;
    --loss) LOSS="$2"; shift 2 ;;
    --churn-interval) CHURN_INTERVAL="$2"; shift 2 ;;
    --churn-min-active) CHURN_MIN_ACTIVE="$2"; shift 2 ;;
    --churn-pjoin) CHURN_PJOIN="$2"; shift 2 ;;
    --churn-pleave) CHURN_PLEAVE="$2"; shift 2 ;;
    --max-nodes) MAX_NODES="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) echo "[ERROR] Unknown argument: $1"; usage ;;
  esac
done

# --- Validate required -------------------------------------------------------
missing=()
[[ -z "$KEYPAIR" ]] && missing+=("--keypair")
[[ -z "$S3_BUCKET" ]] && missing+=("--s3-bucket")
[[ -z "$S3_PREFIX" ]] && missing+=("--s3-prefix")
[[ -z "$VPC_ID" ]] && missing+=("--vpc-id")
[[ -z "$SUBNET_ID" ]] && missing+=("--subnet-id")
[[ -z "$INSTANCE_TYPE" ]] && missing+=("--instance-type")
[[ -z "$SIM_DURATION" ]] && missing+=("--sim-duration")
[[ -z "$QUERY_RATE" ]] && missing+=("--query-rate")
[[ -z "$PARALLEL_MIN" ]] && missing+=("--parallel-min")
[[ -z "$PARALLEL_MAX" ]] && missing+=("--parallel-max")
[[ -z "$DELAY" ]] && missing+=("--delay")
[[ -z "$JITTER" ]] && missing+=("--jitter")
[[ -z "$LOSS" ]] && missing+=("--loss")
[[ -z "$CHURN_INTERVAL" ]] && missing+=("--churn-interval")
[[ -z "$CHURN_MIN_ACTIVE" ]] && missing+=("--churn-min-active")
[[ -z "$CHURN_PJOIN" ]] && missing+=("--churn-pjoin")
[[ -z "$CHURN_PLEAVE" ]] && missing+=("--churn-pleave")
[[ -z "$MAX_NODES" ]] && missing+=("--max-nodes")

if (( ${#missing[@]} > 0 )); then
  echo "[ERROR] Missing required parameters: ${missing[*]}"
  usage
fi

# --- Create Stack ------------------------------------------------------------
echo "[INFO] Creating CloudFormation stack '${STACK_NAME}'..."
aws cloudformation create-stack \
  --stack-name "$STACK_NAME" \
  --template-body "file://$TEMPLATE_FILE" \
  --capabilities CAPABILITY_IAM \
  --parameters \
    ParameterKey=KeyName,ParameterValue="$KEYPAIR" \
    ParameterKey=InstanceType,ParameterValue="$INSTANCE_TYPE" \
    ParameterKey=VpcId,ParameterValue="$VPC_ID" \
    ParameterKey=SubnetId,ParameterValue="$SUBNET_ID" \
    ParameterKey=S3Bucket,ParameterValue="$S3_BUCKET" \
    ParameterKey=S3Prefix,ParameterValue="$S3_PREFIX" \
    ParameterKey=SimDuration,ParameterValue="$SIM_DURATION" \
    ParameterKey=QueryRate,ParameterValue="$QUERY_RATE" \
    ParameterKey=ParallelMin,ParameterValue="$PARALLEL_MIN" \
    ParameterKey=ParallelMax,ParameterValue="$PARALLEL_MAX" \
    ParameterKey=Delay,ParameterValue="$DELAY" \
    ParameterKey=Jitter,ParameterValue="$JITTER" \
    ParameterKey=Loss,ParameterValue="$LOSS" \
    ParameterKey=ChurnInterval,ParameterValue="$CHURN_INTERVAL" \
    ParameterKey=ChurnMinActive,ParameterValue="$CHURN_MIN_ACTIVE" \
    ParameterKey=ChurnPJoin,ParameterValue="$CHURN_PJOIN" \
    ParameterKey=ChurnPLeave,ParameterValue="$CHURN_PLEAVE" \
    ParameterKey=MaxNodes,ParameterValue="$MAX_NODES"

echo "[INFO] Waiting for stack creation to complete..."
aws cloudformation wait stack-create-complete --stack-name "$STACK_NAME"

echo
echo "[SUCCESS] Stack '${STACK_NAME}' created successfully."
aws cloudformation describe-stacks \
  --stack-name "$STACK_NAME" \
  --query "Stacks[0].Outputs" \
  --output table
