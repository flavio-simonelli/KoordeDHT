#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# init-delay.sh
# -----------------------------------------------------------------------------
# This script applies or removes artificial network delay, jitter, and packet loss
# on a Docker bridge network used by your Compose deployment ("koordenet").
#
# It relies on the Linux 'tc netem' facility to emulate real-world network
# conditions between containers connected to the specified Docker network.
#
# Usage:
#   ./init-delay.sh [apply|clear] [--delay Xms] [--jitter Yms] [--loss Z%] [--network NAME] [--timeout N]
#
# Examples:
#   ./init-delay.sh apply --delay 200ms --jitter 50ms --loss 0.1% --network koordenet
#   ./init-delay.sh clear --network koordenet
#
# Notes:
#   - Must be executed after 'docker compose up' (the network must already exist).
#   - Requires sudo privileges (to modify traffic control rules).
#   - Works only for local Docker networks (bridge driver).
# -----------------------------------------------------------------------------

# Default values
ACTION="apply"
DELAY="200ms"
JITTER="50ms"
LOSS="0.0%"
NETWORK="koordenet"
TIMEOUT=60   # seconds

# Usage message
usage() {
  echo "Usage: $0 <apply|clear> [--delay Xms] [--jitter Yms] [--loss Z%] [--network NAME] [--timeout N]"
  echo
  echo "Examples:"
  echo "  $0 apply --delay 200ms --jitter 50ms --loss 0.1% --network koordenet"
  echo "  $0 clear --network koordenet"
  echo
  echo "Options:"
  echo "  apply           Apply network emulation (default)"
  echo "  clear           Remove existing emulation rules"
  echo "  --delay Xms     Average latency to introduce (default: 200ms)"
  echo "  --jitter Yms    Random delay variation (default: 50ms)"
  echo "  --loss Z%       Packet loss percentage (default: 0.0%)"
  echo "  --network NAME  Target Docker network (default: koordenet)"
  echo "  --timeout N     Max seconds to wait for network availability (default: 60)"
  exit 1
}

# Parse arguments
if [[ $# -lt 1 ]]; then
  usage
fi

ACTION="$1"
shift || true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --delay)   DELAY="$2"; shift 2 ;;
    --jitter)  JITTER="$2"; shift 2 ;;
    --loss)    LOSS="$2"; shift 2 ;;
    --network) NETWORK="$2"; shift 2 ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) echo "[ERROR] Unknown option: $1"; usage ;;
  esac
done

# Wait for the Docker network (with timeout)
echo "[INFO] Waiting for Docker network '$NETWORK' (timeout: ${TIMEOUT}s)..."
ELAPSED=0
INTERVAL=2
while ! docker network inspect "$NETWORK" &>/dev/null; do
  sleep "$INTERVAL"
  ELAPSED=$((ELAPSED + INTERVAL))
  if [[ $ELAPSED -ge $TIMEOUT ]]; then
    echo "[ERROR] Network '$NETWORK' not found after ${TIMEOUT}s."
    echo "[HINT] Ensure 'docker compose up -d' has been executed."
    exit 1
  fi
done
echo "[OK] Docker network '$NETWORK' detected."

# Identify the Linux bridge interface
NET_ID=$(docker network inspect "$NETWORK" -f '{{.Id}}' | cut -c1-12)
IFACE="br-$NET_ID"

# Apply or remove delay
case "$ACTION" in
  apply)
    echo "[INFO] Applying delay=${DELAY} ±${JITTER}, loss=${LOSS} on interface ${IFACE}"
    sudo tc qdisc del dev "$IFACE" root 2>/dev/null || true
    sudo tc qdisc add dev "$IFACE" root netem \
      delay "$DELAY" "$JITTER" distribution normal \
      loss "$LOSS"
    echo "[OK] Network emulation applied on Docker network '$NETWORK' (${IFACE})"
    ;;

  clear)
    echo "[INFO] Clearing network emulation rules from ${IFACE}..."
    sudo tc qdisc del dev "$IFACE" root 2>/dev/null || true
    echo "[OK] Network restored to normal conditions"
    ;;

  *)
    usage
    ;;
esac
