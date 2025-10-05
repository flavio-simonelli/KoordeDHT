#!/usr/bin/env bash
set -euo pipefail

LOG_FILE="/var/log/gen_compose.log"
exec > >(tee -a "$LOG_FILE") 2>&1

# -----------------------------------------------------------------------------
# Koorde Cluster Compose Generator
# -----------------------------------------------------------------------------
# Generates a docker-compose file with one bootstrap node, one client,
# and N peer nodes.
#
# Example:
#   ./gen-compose.sh --peers 8 \
#       --sim-duration 10s \
#       --query-rate 5 \
#       --parallelism-min 1 \
#       --parallelism-max 4
# -----------------------------------------------------------------------------

usage() {
  echo "Usage: $0 --peers <N> [--sim-duration <T>] [--query-rate <R>] [--parallelism-min <N>] [--parallelism-max <N>]"
  echo
  echo "Options:"
  echo "  --peers <N>              Number of peer containers to generate"
  echo "  --sim-duration <T>       Duration of tester simulation (default: 10s)"
  echo "  --query-rate <R>         Query rate for tester (req/s, default: 1)"
  echo "  --parallelism-min <N>    Minimum parallelism (default: 1)"
  echo "  --parallelism-max <N>    Maximum parallelism (default: 1)"
  echo
  echo "Example:"
  echo "  $0 --peers 5 --sim-duration 60s --query-rate 10 --parallelism-min 2 --parallelism-max 5"
  exit 1
}

# --- Default values ---
PEERS=""
SIM_DURATION=""
QUERY_RATE=""
PARALLELISM_MIN=""
PARALLELISM_MAX=""

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --peers)            PEERS="$2"; shift 2 ;;
    --sim-duration)     SIM_DURATION="$2"; shift 2 ;;
    --query-rate)       QUERY_RATE="$2"; shift 2 ;;
    --parallelism-min)  PARALLELISM_MIN="$2"; shift 2 ;;
    --parallelism-max)  PARALLELISM_MAX="$2"; shift 2 ;;
    -h|--help)          usage ;;
    *) echo "Unknown argument: $1"; usage ;;
  esac
done

if [[ -z "$PEERS" || -z "$SIM_DURATION" || -z "$QUERY_RATE" || -z "$PARALLELISM_MIN" || -z "$PARALLELISM_MAX" ]]; then
  echo "[ERROR] Missing required arguments."
  echo "You must specify all of: --peers, --sim-duration, --query-rate, --parallelism-min, --parallelism-max"
  echo
  usage
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="$SCRIPT_DIR/docker-compose.test.template.yml"
OUT="$SCRIPT_DIR/docker-compose.test.generated.yml"

# Copy static parts (everything before '# Template for peers')
awk '/# Template for peers/ {exit} {print}' "$TEMPLATE" > "$OUT"

# Replace placeholders in the bootstrap and tester sections before peers
sed -i \
  -e "s/\${SIM_DURATION}/$SIM_DURATION/g" \
  -e "s/\${QUERY_RATE}/$QUERY_RATE/g" \
  -e "s/\${QUERY_PARALLELISM_MIN}/$PARALLELISM_MIN/g" \
  -e "s/\${QUERY_PARALLELISM_MAX}/$PARALLELISM_MAX/g" \
  "$OUT"

# Append generated peers
echo "  # Peers" >> "$OUT"
for i in $(seq 1 "$PEERS"); do
  echo "  koorde-peer-$i:" >> "$OUT"

  awk '/peer-template:/,/^$/' "$TEMPLATE" \
    | tail -n +2 \
    | sed "s/\$ID/$i/g" \
    | sed 's/^  /  /' >> "$OUT"

  echo "" >> "$OUT"
done

echo "[SUCCESS] Generated compose file:"
echo "  -> $OUT"
echo
echo "Tester configuration:"
echo "  SIM_DURATION=$SIM_DURATION"
echo "  QUERY_RATE=$QUERY_RATE"
echo "  QUERY_PARALLELISM_MIN=$PARALLELISM_MIN"
echo "  QUERY_PARALLELISM_MAX=$PARALLELISM_MAX"
