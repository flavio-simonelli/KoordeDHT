#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 --nodes <N> --base-port <P> --version <VERSION> --mode <public|private>"
  exit 1
}

NODES=""
BASE_PORT=""
VERSION=""
MODE="private"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --nodes) NODES="$2"; shift 2 ;;
    --base-port) BASE_PORT="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --mode) MODE="$2"; shift 2 ;;
    *) usage ;;
  esac
done

if [[ -z "$NODES" || -z "$BASE_PORT" || -z "$VERSION" ]]; then
  usage
fi

# Recupera IP EC2
if [[ "$MODE" == "public" ]]; then
  NODE_HOST=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
else
  NODE_HOST=$(curl -s http://169.254.169.254/latest/meta-data/local-ipv4)
fi

OUT="docker-compose.koorde_nodes.generated.yml"

# Scrive header
cat > "$OUT" <<EOF
version: "3.9"

networks:
  koordenet:
    driver: bridge

services:
EOF

# Genera blocchi dai template
for i in $(seq 1 $NODES); do
  HOST_PORT=$((BASE_PORT + i - 1))

  # sostituisci i placeholder nel template
  sed \
    -e "s/\$ID/$i/g" \
    -e "s/\${VERSION}/$VERSION/g" \
    -e "s/\$VERSION/$VERSION/g" \
    -e "s/\$HOST_PORT/$HOST_PORT/g" \
    -e "s/\$NODE_HOST/$NODE_HOST/g" \
    docker-compose.koorde_nodes.template.yml >> "$OUT"

  echo "" >> "$OUT"
done

echo "✅ File $OUT generato con successo"
