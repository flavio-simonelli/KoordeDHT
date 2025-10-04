#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Koorde DHT Cluster Deployer
# -----------------------------------------------------------------------------
# This script launches multiple EC2 instances with CloudFormation.
# Each EC2 instance runs "bootstrap.sh" to start a local Koorde cluster
# with the given number of nodes.
# -----------------------------------------------------------------------------

usage() {
  echo "Usage: $0 --instances <M> --nodes <N> --base-port <P> --version <minimal|medium|strong> --mode <public|private> --zone-id <ZONE_ID> --suffix <SUFFIX> --s3-bucket <BUCKET> --s3-prefix <PREFIX> --key-name <KEY> --instance-type <TYPE> --vpc-id <VPC> --subnet-id <SUBNET>"
  echo
  echo "Options (all required):"
  echo "  --instances <M>     Number of EC2 instances to launch"
  echo "  --nodes <N>         Number of Koorde containers per instance"
  echo "  --base-port <P>     Starting port for the first container (on each instance)"
  echo "  --version           Docker image version (minimal|medium|strong)"
  echo "  --mode              Node host mode (public|private)"
  echo "  --zone-id           Route53 Hosted Zone ID"
  echo "  --suffix            DNS suffix (e.g. dht.local)"
  echo "  --region          AWS region for Route53 (default: us-east-1)"
  echo "  --s3-bucket         S3 bucket containing scripts"
  echo "  --s3-prefix         S3 prefix/folder (e.g. scripts)"
  echo "  --key-name          EC2 KeyPair name"
  echo "  --instance-type     EC2 instance type (e.g. t3.micro)"
  echo "  --vpc-id            VPC ID where instances will be created"
  echo "  --subnet-id         Subnet ID for instances"
  exit 1
}

# -----------------------------------------------------------------------------
# Parse arguments
# -----------------------------------------------------------------------------
INSTANCES="" NODES="" BASE_PORT="" VERSION="" MODE="" ROUTE53_ZONE_ID="" ROUTE53_SUFFIX=""
S3_BUCKET="" S3_PREFIX="" KEY_NAME="" INSTANCE_TYPE="" VPC_ID="" SUBNET_ID="" ROUTE53_REGION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --instances) INSTANCES="$2"; shift 2 ;;
    --nodes) NODES="$2"; shift 2 ;;
    --base-port) BASE_PORT="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --mode) MODE="$2"; shift 2 ;;
    --zone-id) ROUTE53_ZONE_ID="$2"; shift 2 ;;
    --suffix) ROUTE53_SUFFIX="$2"; shift 2 ;;
    --region) ROUTE53_REGION="$2"; shift 2 ;;
    --s3-bucket) S3_BUCKET="$2"; shift 2 ;;
    --s3-prefix) S3_PREFIX="$2"; shift 2 ;;
    --key-name) KEY_NAME="$2"; shift 2 ;;
    --instance-type) INSTANCE_TYPE="$2"; shift 2 ;;
    --vpc-id) VPC_ID="$2"; shift 2 ;;
    --subnet-id) SUBNET_ID="$2"; shift 2 ;;
    *) usage ;;
  esac
done

# -----------------------------------------------------------------------------
# Validation
# -----------------------------------------------------------------------------
if [[ -z "$INSTANCES" || -z "$NODES" || -z "$BASE_PORT" || -z "$VERSION" || -z "$MODE" || -z "$ROUTE53_ZONE_ID" || -z "$ROUTE53_SUFFIX" || -z "$ROUTE53_REGION" || -z "$S3_BUCKET" || -z "$S3_PREFIX" || -z "$KEY_NAME" || -z "$INSTANCE_TYPE" || -z "$VPC_ID" || -z "$SUBNET_ID" ]]; then
  echo "[ERROR] Missing required parameters"
  usage
fi

if ! [[ "$INSTANCES" =~ ^[0-9]+$ ]] || [[ "$INSTANCES" -le 0 ]]; then
  echo "[ERROR] --instances must be a positive integer"
  exit 1
fi

if ! [[ "$NODES" =~ ^[0-9]+$ ]] || [[ "$NODES" -le 0 ]]; then
  echo "[ERROR] --nodes must be a positive integer"
  exit 1
fi

if ! [[ "$BASE_PORT" =~ ^[0-9]+$ ]] || [[ "$BASE_PORT" -lt 1024 || "$BASE_PORT" -gt 65535 ]]; then
  echo "[ERROR] --base-port must be a valid port number (1024-65535)"
  exit 1
fi

if [[ "$VERSION" != "minimal" && "$VERSION" != "medium" && "$VERSION" != "strong" ]]; then
  echo "[ERROR] --version must be one of: minimal, medium, strong"
  exit 1
fi

if [[ "$MODE" != "public" && "$MODE" != "private" ]]; then
  echo "[ERROR] --mode must be either public or private"
  exit 1
fi

# -----------------------------------------------------------------------------
# Deploy M stacks (one per EC2 instance)
# -----------------------------------------------------------------------------
NODE_BASE_PORT=$((BASE_PORT))
NODE_MAX_PORT=$((NODE_BASE_PORT + NODES - 1))

for i in $(seq 1 "$INSTANCES"); do

  STACK_NAME="koorde-instance-$i"

  echo "[INFO] Launching CloudFormation stack: $STACK_NAME"
  echo "       -> EC2 instance with $NODES containers"
  echo "       -> Ports ${NODE_BASE_PORT}-${NODE_MAX_PORT}"

  aws cloudformation create-stack \
    --stack-name "$STACK_NAME" \
    --template-body file://koorde.yml \
    --capabilities CAPABILITY_IAM \
    --parameters \
      ParameterKey=InstanceType,ParameterValue=$INSTANCE_TYPE \
      ParameterKey=KeyName,ParameterValue=$KEY_NAME \
      ParameterKey=VpcId,ParameterValue=$VPC_ID \
      ParameterKey=SubnetId,ParameterValue=$SUBNET_ID \
      ParameterKey=S3Bucket,ParameterValue=$S3_BUCKET \
      ParameterKey=S3Prefix,ParameterValue=$S3_PREFIX \
      ParameterKey=Nodes,ParameterValue=$NODES \
      ParameterKey=BasePort,ParameterValue=$NODE_BASE_PORT \
      ParameterKey=MaxPort,ParameterValue=$NODE_MAX_PORT \
      ParameterKey=Version,ParameterValue=$VERSION \
      ParameterKey=Mode,ParameterValue=$MODE \
      ParameterKey=Route53ZoneId,ParameterValue=$ROUTE53_ZONE_ID \
      ParameterKey=Route53Suffix,ParameterValue=$ROUTE53_SUFFIX \
      ParameterKey=Route53Region,ParameterValue=$ROUTE53_REGION
done

echo "[SUCCESS] Launched $INSTANCES EC2 instances, each running $NODES Koorde containers."
