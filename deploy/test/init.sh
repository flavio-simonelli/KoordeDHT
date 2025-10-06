#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# KoordeDHT Full Simulation Orchestrator
# -----------------------------------------------------------------------------
# Coordinates Compose generation, delay, churn and cleanup.
#
# Usage:
#   ./init.sh \
#     --sim-duration 60s \
#     --query-rate 0.5 \
#     --parallel-min 1 \
#     --parallel-max 5 \
#     --delay 200ms \
#     --jitter 50ms \
#     --loss 0.1% \
#     --churn-prefix node \
#     --churn-interval 20 \
#     --churn-min-active 3 \
#     --churn-pjoin 0.4 \
#     --churn-pleave 0.3 \
#     --max-nodes 5
# -----------------------------------------------------------------------------

LOG_FILE="/var/log/test/init.log"
exec > >(tee -a "$LOG_FILE") 2>&1

NETWORK="test-koordenet"
TEMPLATE="docker-compose.template.yml"
GENERATED="docker-compose.generated.yml"
CHURN_PREFIX="test-node"

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

# usage
usage() {
  echo
  echo "Usage:"
  echo "  $0 --sim-duration <time> --query-rate <rate> --parallel-min <n> --parallel-max <n> \\"
  echo "     --delay <Xms> --jitter <Yms> --loss <Z%> \\"
  echo "     --churn-prefix <prefix> --churn-interval <sec> --churn-min-active <n> \\"
  echo "     --churn-pjoin <p> --churn-pleave <p> --max-nodes <n>"
  echo
  echo "Example:"
  echo "  $0 --sim-duration 1m --query-rate 0.5 --parallel-min 1 --parallel-max 5 \\"
  echo "     --delay 200ms --jitter 50ms --loss 0.1% \\"
  echo "     --churn-prefix node --churn-interval 20 --churn-min-active 3 \\"
  echo "     --churn-pjoin 0.4 --churn-pleave 0.3 --max-nodes 10"
  echo
  exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
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

# validate parameters
missing=()
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

# check required scripts exist
for script in gen_compose.sh network_delay.sh churn.sh; do
  [[ -x "$script" ]] || { echo "[ERROR] Required script '$script' not found or not executable."; exit 1; }
done

# check template exists
[[ -f "$TEMPLATE" ]] || { echo "[ERROR] Template file '$TEMPLATE' not found."; exit 1; }

# check docker installed
command -v docker >/dev/null 2>&1 || { echo "[ERROR] docker not found (install docker)"; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { echo "[ERROR] docker-compose not found (install docker-compose)"; exit 1; }
command -v tc >/dev/null 2>&1 || { echo "[ERROR] tc not found (install iproute2)"; exit 1; }

echo "[INFO] Removing previous output..."
rm -f results/output.log

# generate compose file
echo "[INFO] Generating docker-compose file..."
./gen_compose.sh \
  --sim-duration "$SIM_DURATION" \
  --query-rate "$QUERY_RATE" \
  --query-parallelism-min "$PARALLEL_MIN" \
  --query-parallelism-max "$PARALLEL_MAX"

echo "[OK] Generated $GENERATED."

# start docker compose
echo "[INFO] Starting Docker Compose..."
docker compose -f "$GENERATED" up -d --scale node="$MAX_NODES"
echo "[OK] Docker Compose started."

# apply network delay
echo "[INFO] Applying network delay..."
NETWORK=$(docker network ls --format '{{.Name}}' | grep 'koordenet$' | head -n1)
./network_delay.sh apply \
  --delay "$DELAY" \
  --jitter "$JITTER" \
  --loss "$LOSS" \
  --network "$NETWORK"
echo "[OK] Network delay applied on the network $NETWORK"

echo "[INFO] Starting Pumba network delay daemon..."
docker rm -f pumba-delay >/dev/null 2>&1 || true
docker run -d --name pumba-delay \
  --network host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gaiaadm/pumba netem \
  --duration "$SIM_DURATION" \
  delay --time "${DELAY/ms/}" --jitter "${JITTER/ms/}" \
  re2:.*node
echo "[OK] Pumba delay running for $SIM_DURATION (delay=$DELAY, jitter=$JITTER, loss=$LOSS)."


# start churn controller
echo "[INFO] Starting churn controller..."
./churn.sh apply \
  -p "$CHURN_PREFIX" \
  -i "$CHURN_INTERVAL" \
  -m "$CHURN_MIN_ACTIVE" \
  -j "$CHURN_PJOIN" \
  -l "$CHURN_PLEAVE" &
CHURN_PID=$!
echo "[OK] Churn controller started with PID $CHURN_PID."

# wait for simulation duration
echo "[INFO] Simulation running for $SIM_DURATION..."
sleep "$SIM_DURATION"
echo "[INFO] Simulation duration elapsed."

# stop churn controller
if kill -0 "$CHURN_PID" &>/dev/null; then
  echo "[INFO] Stopping churn controller..."
  kill "$CHURN_PID"
  wait "$CHURN_PID" 2>/dev/null || true
  echo "[OK] Churn controller stopped."
else
  echo "[WARN] Churn controller process not found."
fi

# remove network delay
echo "[INFO] Removing network delay..."
./network_delay.sh clear --network "$NETWORK"
echo "[OK] Network delay removed."

# stop and remove containers
echo "[INFO] Stopping and removing all containers..."
docker compose -f "$GENERATED" down
echo "[OK] All containers stopped and removed."

echo "[SUCCESS] Simulation completed successfully."
