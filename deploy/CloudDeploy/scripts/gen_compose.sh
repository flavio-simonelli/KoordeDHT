#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Koorde Compose Generator Script
# -----------------------------------------------------------------------------
# This script generates a docker-compose.yml file for a cluster of Koorde nodes.
# Steps:
#   1. Validate input parameters
#   2. Fetch EC2 instance IP (public or private)
#   3. Replace template placeholders with actual values
#   4. Write final docker-compose file
# -----------------------------------------------------------------------------

LOG_FILE="/var/log/koorde-gen-compose.log"
exec > >(tee -a "$LOG_FILE") 2>&1

usage() {
  echo "[USAGE]: $0 --nodes <N> --base-port <P> --version <minimal|medium|strong> --mode <public|private> --zone-id <ZONE_ID> --suffix <SUFFIX>"
  echo
  echo "Generates docker-compose.koorde_nodes.generated.yml to launch a Koorde DHT cluster on EC2."
  echo
  echo "Options:"
  echo "  --nodes <N>         Number of nodes to generate (e.g. 5)."
  echo "  --base-port <P>     Starting port for the first node (e.g. 4000)."
  echo "                      Each subsequent node will increment the port by 1 (4001, 4002...)."
  echo "  --version           Docker image version (allowed: minimal | medium | strong)."
  echo "  --mode              Use EC2 public or private IP (default: private)."
  echo "  --zone-id           Route53 Hosted Zone ID for registration."
  echo "  --suffix            DNS suffix for node registration (e.g. dht.local)."
  exit 1
}

# Default values
NODES=""
BASE_PORT=""
VERSION=""
MODE="private"
ROUTE53_ZONE_ID=""
ROUTE53_SUFFIX=""

# Parse CLI arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --nodes) NODES="$2"; shift 2 ;;
    --base-port) BASE_PORT="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --mode) MODE="$2"; shift 2 ;;
    --zone-id) ROUTE53_ZONE_ID="$2"; shift 2 ;;
    --suffix) ROUTE53_SUFFIX="$2"; shift 2 ;;
    *) usage ;;
  esac
done

# Parameter validation
if [[ -z "$NODES" || -z "$BASE_PORT" || -z "$VERSION" || -z "$ROUTE53_ZONE_ID" || -z "$ROUTE53_SUFFIX" ]]; then
  echo "[ERROR]: Missing required parameters"
  usage
fi

if ! [[ "$NODES" =~ ^[0-9]+$ ]] || [[ "$NODES" -le 0 ]]; then
  echo "[ERROR]: --nodes must be a positive integer"
  usage
fi

if ! [[ "$BASE_PORT" =~ ^[0-9]+$ ]] || [[ "$BASE_PORT" -lt 1024 || "$BASE_PORT" -gt 65535 ]]; then
  echo "[ERROR]: --base-port must be a valid port number (1024-65535)"
  usage
fi

if [[ "$VERSION" != "minimal" && "$VERSION" != "medium" && "$VERSION" != "strong" ]]; then
  echo "[ERROR]: --version must be one of: minimal, medium, strong"
  usage
fi

if [[ "$MODE" != "public" && "$MODE" != "private" ]]; then
  echo "[ERROR]: --mode must be either public or private"
  usage
fi

# Determine EC2 IP (metadata service)
if [[ "$MODE" == "public" ]]; then
  NODE_HOST=$(ec2-metadata -v | awk '{print $2}')
else
  NODE_HOST=$(ec2-metadata -o | awk '{print $2}')
fi

if [[ -z "$NODE_HOST" ]]; then
  echo "[ERROR]: Failed to fetch EC2 IP from metadata service"
  exit 1
else
  echo "[INFO]: Using EC2 IP address: $NODE_HOST (mode=$MODE)"
fi

# File paths
TEMPLATE="docker-compose.koorde_nodes.template.yml"
OUT="docker-compose.koorde_nodes.generated.yml"

# Write docker-compose header
cat > "$OUT" <<EOF
networks:
  koordenet:
    driver: bridge

services:
EOF

# Generate node definitions from template
for i in $(seq 1 "$NODES"); do
  NODE_PORT=$((BASE_PORT + i - 1))
  echo "[STEP] Generating service block for node-$i (port $NODE_PORT)"

  # shellcheck disable=SC2129
  echo "  node-$i:" >> "$OUT"

  sed \
    -e "s/\$ID/$i/g" \
    -e "s/\${VERSION}/$VERSION/g" \
    -e "s/\${NODE_HOST}/$NODE_HOST/g" \
    -e "s/\${NODE_PORT}/$NODE_PORT/g" \
    -e "s/\${ROUTE53_ZONE_ID}/$ROUTE53_ZONE_ID/g" \
    -e "s/\${ROUTE53_SUFFIX}/$ROUTE53_SUFFIX/g" \
    "$TEMPLATE" | sed 's/^/    /' >> "$OUT"

  echo "" >> "$OUT"
done

# Completed
echo "[SUCCESS]: docker-compose file generated -> $OUT"
