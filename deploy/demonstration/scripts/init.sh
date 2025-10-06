#!/bin/bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Koorde EC2 Bootstrap Script
# -----------------------------------------------------------------------------
# This script configures an EC2 instance to run a Koorde DHT cluster.
# Steps:
#   1. Download required scripts and templates from S3
#   2. Install Docker & Docker Compose
#   3. Generate a docker-compose.yml for Koorde nodes
#   4. Launch containers using Docker Compose
# -----------------------------------------------------------------------------

# Log file for troubleshooting
LOG_FILE="/var/log/koorde-init.log"
exec > >(tee -a "$LOG_FILE") 2>&1

usage() {
  echo "[USAGE]: $0"
  echo
  echo "Bootstrap script to configure an EC2 instance as a Koorde DHT node cluster."
  echo
  echo "This script downloads required scripts from S3, installs Docker and"
  echo "Docker Compose, generates a docker-compose.yml, and starts the Koorde nodes."
  echo
  echo "Required environment variables (must be set before running):"
  echo "  NODES                Number of Koorde nodes to start (e.g. 3, 5, 10)"
  echo "  BASE_PORT            Starting port for the first node (e.g. 4000)."
  echo "                       Each subsequent node increments by +1 (4001, 4002...)"
  echo "  VERSION              Docker image version (minimal | medium | strong)"
  echo "  MODE                 IP mode for node advertising (public | private)"
  echo "  ROUTE53_ZONE_ID     Route53 Hosted Zone ID"
  echo "  ROUTE53_SUFFIX      Route53 suffix for node registration (e.g. dht.local)"
  echo "  ROUTE53_REGION     AWS region for Route53 (default: us-east-1)"
  echo "  S3_BUCKET            S3 bucket containing Koorde deployment scripts"
  echo
  echo "Optional variables:"
  echo "  S3_PREFIX            Folder inside the S3 bucket where scripts are stored"
  echo "                       Default: scripts"
  exit 1
}

if [[ -z "$NODES" || -z "$BASE_PORT" || -z "$VERSION" || -z "$MODE" || -z "$ROUTE53_ZONE_ID" || -z "$ROUTE53_SUFFIX" || -z "$S3_BUCKET" ]]; then
  echo "[ERROR] Missing required parameters"
  usage
fi

echo "[INFO] Starting Koorde EC2 initialization..."

# Configurable variables (can be overridden via environment)
NODES=${NODES:-3}           # Number of Koorde nodes
BASE_PORT=${BASE_PORT:-4000} # Starting port
VERSION=${VERSION:-strong}   # Docker image version (minimal|medium|strong)
MODE=${MODE:-private}        # IP mode: private or public
ROUTE53_ZONE_ID=${ROUTE53_ZONE_ID:-""}   # Route53 Hosted Zone ID
ROUTE53_SUFFIX=${ROUTE53_SUFFIX:-""}     # DNS suffix for registration
ROUTE53_REGION=${ROUTE53_REGION:-"us-east-1"} # AWS region for Route53

S3_BUCKET=${S3_BUCKET:-"koorde-deploy"}   # S3 bucket containing scripts
S3_PREFIX=${S3_PREFIX:-"scripts"}         # Prefix/folder inside bucket

WORKDIR="/opt/koorde"       # Working directory on EC2
mkdir -p "$WORKDIR"

# Download scripts and templates from S3
echo "[STEP] Downloading deployment scripts from S3 (bucket=$S3_BUCKET, prefix=$S3_PREFIX)"
aws s3 cp "s3://${S3_BUCKET}/${S3_PREFIX}/install_docker.sh" "$WORKDIR/install-docker.sh" || { echo "[ERROR] Failed to download install_docker.sh"; exit 1; }
aws s3 cp "s3://${S3_BUCKET}/${S3_PREFIX}/gen_compose.sh" "$WORKDIR/gen-compose.sh" || { echo "[ERROR] Failed to download gen_compose.sh"; exit 1; }
aws s3 cp "s3://${S3_BUCKET}/${S3_PREFIX}/docker-compose.koorde_nodes.template.yml" "$WORKDIR/docker-compose.koorde_nodes.template.yml" || { echo "[ERROR] Failed to download template"; exit 1; }
aws s3 cp "s3://${S3_BUCKET}/${S3_PREFIX}/common_node.env" "/home/ec2-user/common_node.env" || { echo "[ERROR] Failed to download common_node.env"; exit 1; }

chmod +x "$WORKDIR/install-docker.sh" "$WORKDIR/gen-compose.sh"

# Install Docker and Docker Compose
echo "[STEP] Installing Docker & Docker Compose..."
bash "$WORKDIR/install-docker.sh"

# Generate docker-compose file for Koorde nodes
echo "[STEP] Generating docker-compose configuration for $NODES nodes..."
bash "$WORKDIR/gen-compose.sh" \
  --nodes "$NODES" \
  --base-port "$BASE_PORT" \
  --version "$VERSION" \
  --mode "$MODE" \
  --zone-id "$ROUTE53_ZONE_ID" \
  --suffix "$ROUTE53_SUFFIX" \
  --region "$ROUTE53_REGION" \

# Deploy Koorde cluster with Docker Compose
TARGET_COMPOSE="/home/ec2-user/docker-compose.yml"
cp "$WORKDIR/docker-compose.koorde_nodes.generated.yml" "$TARGET_COMPOSE"
chown ec2-user:ec2-user "$TARGET_COMPOSE"
chmod 644 "$TARGET_COMPOSE"

echo "[STEP] Launching Koorde nodes with Docker Compose..."
cd /home/ec2-user
docker-compose up -d || { echo "[ERROR] Failed to start Koorde containers"; exit 1; }

# Completed
echo "[SUCCESS] Koorde initialization complete."
echo "[INFO] Nodes are running under Docker Compose in /home/ec2-user"
echo "[NOTE] To manage the cluster, run: cd /home/ec2-user && docker-compose ps"
