#!/usr/bin/env bash
set -euo pipefail

LOG_FILE="/var/log/init_test.log"
exec > >(tee -a "$LOG_FILE") 2>&1

# -----------------------------------------------------------------------------
# KoordeDHT Automated Test Orchestrator
# -----------------------------------------------------------------------------
# Runs a full Koorde test including:
#   1. Docker Compose generation
#   2. Cluster startup
#   3. Network delay/jitter/loss setup
#   4. Churn simulation
#
# Requirements:
#   - docker & docker compose installed
#   - scripts: gen_compose.sh, network_delay.sh, churn.sh available
#
# Example:
#   ./init_test.sh \
#       --peers 10 \
#       --delay 200ms --jitter 50ms --loss 0.1% \
#       --churn-interval 20 --churn-join 0.4 --churn-leave 0.3 --min-active 3 \
#       --sim-duration 120s --query-rate 5 --parallelism-min 2 --parallelism-max 5
# -----------------------------------------------------------------------------

usage() {
  echo "Usage: $0 --peers N [--prefix PFX] [--delay Xms] [--jitter Yms] [--loss Z%]"
  echo "          [--churn-interval S] [--churn-join P] [--churn-leave P] [--min-active N]"
  echo "          [--sim-duration T] [--query-rate R] [--parallelism-min N] [--parallelism-max N]"
  echo
  echo "Example:"
  echo "  $0 --peers 8 --delay 150ms --jitter 40ms --loss 0.05% \\"
  echo "     --churn-interval 15 --churn-join 0.4 --churn-leave 0.3 --min-active 3 \\"
  echo "     --sim-duration 60s --query-rate 5 --parallelism-min 2 --parallelism-max 5"
  exit 1
}

# --- Default values ---
PEERS=
PREFIX="koorde-peer"
DELAY="200ms"
JITTER="50ms"
LOSS="0.0%"
CHURN_INTERVAL=15
CHURN_JOIN=0.5
CHURN_LEAVE=0.5
CHURN_MIN_ACTIVE=2
NETWORK="test_koordenet"

SIM_DURATION="10s"
QUERY_RATE="1"
PARALLELISM_MIN="1"
PARALLELISM_MAX="1"

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --peers)            PEERS="$2"; shift 2 ;;
    --prefix)           PREFIX="$2"; shift 2 ;;
    --delay)            DELAY="$2"; shift 2 ;;
    --jitter)           JITTER="$2"; shift 2 ;;
    --loss)             LOSS="$2"; shift 2 ;;
    --churn-interval)   CHURN_INTERVAL="$2"; shift 2 ;;
    --churn-join)       CHURN_JOIN="$2"; shift 2 ;;
    --churn-leave)      CHURN_LEAVE="$2"; shift 2 ;;
    --min-active)       CHURN_MIN_ACTIVE="$2"; shift 2 ;;
    --sim-duration)     SIM_DURATION="$2"; shift 2 ;;
    --query-rate)       QUERY_RATE="$2"; shift 2 ;;
    --parallelism-min)  PARALLELISM_MIN="$2"; shift 2 ;;
    --parallelism-max)  PARALLELISM_MAX="$2"; shift 2 ;;
    -h|--help)          usage ;;
    *) echo "[ERROR] Unknown argument: $1"; usage ;;
  esac
done

[[ -z "$PEERS" ]] && { echo "[ERROR] Missing --peers"; usage; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- Generate Docker Compose ---
echo "==> [STEP] Generating docker-compose for ${PEERS} peers..."
"${SCRIPT_DIR}/gen_compose.sh" \
  --peers "$PEERS" \
  --sim-duration "$SIM_DURATION" \
  --query-rate "$QUERY_RATE" \
  --parallelism-min "$PARALLELISM_MIN" \
  --parallelism-max "$PARALLELISM_MAX"

# --- Start cluster ---
echo "==> [STEP] Starting cluster..."
docker compose -f "${SCRIPT_DIR}/docker-compose.test.generated.yml" up -d

echo "[INFO] Waiting for containers to stabilize..."
sleep 10
docker ps --format "table {{.Names}}\t{{.Status}}" | grep "${PREFIX}" || true

# --- Apply network delay ---
echo "==> [STEP] Applying network delay..."
"${SCRIPT_DIR}/network_delay.sh" apply \
  --delay "$DELAY" \
  --jitter "$JITTER" \
  --loss "$LOSS" \
  --network "$NETWORK"

# --- Step 4: Start churn controller ---
echo "==> [STEP] Starting churn controller..."
"${SCRIPT_DIR}/churn.sh" \
  -p "$PREFIX" \
  -i "$CHURN_INTERVAL" \
  -m "$CHURN_MIN_ACTIVE" \
  -j "$CHURN_JOIN" \
  -l "$CHURN_LEAVE" &

CHURN_PID=$!
echo "[OK] Churn controller running in background (PID: $CHURN_PID)"

# Wait a few seconds and check client logs
echo "[INFO] Checking tester client status..."
sleep 3

CLIENT_STATUS=$(docker inspect -f '{{.State.Status}}' koorde-tester 2>/dev/null || echo "not_found")

if [[ "$CLIENT_STATUS" == "running" ]]; then
  echo "[OK] Tester client is running."
elif [[ "$CLIENT_STATUS" == "exited" ]]; then
  echo "[WARN] Tester client exited early — fetching last logs..."
  docker logs koorde-tester || echo "[WARN] No logs found from tester."
elif [[ "$CLIENT_STATUS" == "not_found" ]]; then
  echo "[ERROR] Tester container 'koorde-tester' not found! Check compose generation."
else
  echo "[WARN] Tester client state: $CLIENT_STATUS"
fi

# --- Summary ---
echo "------------------------------------------------------------"
echo "[SUMMARY]"
echo "  Peers:              $PEERS"
echo "  Prefix:             $PREFIX"
echo "  Network:            $NETWORK"
echo "  Delay:              $DELAY ± $JITTER"
echo "  Loss:               $LOSS"
echo "  Churn interval:     ${CHURN_INTERVAL}s"
echo "  P(join):            $CHURN_JOIN"
echo "  P(leave):           $CHURN_LEAVE"
echo "  Min active:         $CHURN_MIN_ACTIVE"
echo "  Sim duration:       $SIM_DURATION"
echo "  Query rate:         $QUERY_RATE"
echo "  Parallelism min:    $PARALLELISM_MIN"
echo "  Parallelism max:    $PARALLELISM_MAX"
echo "------------------------------------------------------------"
echo "[INFO] Cluster, delay and churn are active. Use Ctrl+C to stop."
echo

# --- Cleanup handler ---
trap "echo; echo '[CLEANUP] Stopping churn & clearing delay...'; kill $CHURN_PID 2>/dev/null; ${SCRIPT_DIR}/network_delay.sh clear --network $NETWORK; exit 0" SIGINT SIGTERM

# --- Tail logs ---
docker compose logs -f
