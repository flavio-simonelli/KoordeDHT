#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Koorde DHT Churn Controller
# -----------------------------------------------------------------------------
# Randomly stops and starts Docker containers to simulate churn.
# Works only with containers already defined in docker-compose.
#
# Commands:
#   apply   Start churn loop (default)
#   clear   Stop all churn processes and restore containers
#
# Stop options (during apply mode):
#   - Press Ctrl+C (SIGINT)
# -----------------------------------------------------------------------------

# Redirect stdout/stderr both to console and log
LOG_FILE="/var/log/test/churn.log"
exec > >(tee -a "$LOG_FILE") 2>&1

# Variables
ACTION=""
PREFIX=""
INTERVAL=""
MIN_ACTIVE=""
P_JOIN=""
P_LEAVE=""


# usage message
usage() {
  echo
  echo "Usage:"
  echo "  $0 <apply|clear> -p PREFIX -i INTERVAL -m MIN_ACTIVE -j P_JOIN -l P_LEAVE"
  echo
  echo "Options:"
  echo "  apply           Start churn loop"
  echo "  clear           Stop all churn loops and restore containers"
  echo "  -p PREFIX       Prefix of container names (e.g. node)"
  echo "  -i INTERVAL     Interval (seconds) between churn events"
  echo "  -m MIN_ACTIVE   Minimum number of active containers to keep"
  echo "  -j P_JOIN       Probability of performing a join (0.0–1.0)"
  echo "  -l P_LEAVE      Probability of performing a leave (0.0–1.0)"
  echo "  -h              Show this help message"
  echo
  echo "Examples:"
  echo "  $0 apply -p node -i 20 -m 3 -j 0.4 -l 0.3"
  echo "  $0 clear -p node"
  echo
  exit 1
}

# --- Parse options ---
if [[ $# -lt 1 ]]; then
  usage
fi

if [[ "$1" =~ ^(apply|clear)$ ]]; then
  ACTION="$1"
  shift
else
  echo "[ERROR] First argument must be 'apply' or 'clear'."
  usage
fi

while getopts ":p:i:m:j:l:h" opt; do
  case ${opt} in
    p) PREFIX="${OPTARG}" ;;
    i) INTERVAL="${OPTARG}" ;;
    m) MIN_ACTIVE="${OPTARG}" ;;
    j) P_JOIN="${OPTARG}" ;;
    l) P_LEAVE="${OPTARG}" ;;
    h) usage ;;
    \?) echo "[ERROR] Invalid option: -$OPTARG"; usage ;;
    :) echo "[ERROR] Option -$OPTARG requires an argument."; usage ;;
  esac
done

missing=()

if [[ "$ACTION" == "apply" ]]; then
  [[ -z "$PREFIX" ]] && missing+=("-p PREFIX")
  [[ -z "$INTERVAL" ]] && missing+=("-i INTERVAL")
  [[ -z "$MIN_ACTIVE" ]] && missing+=("-m MIN_ACTIVE")
  [[ -z "$P_JOIN" ]] && missing+=("-j P_JOIN")
  [[ -z "$P_LEAVE" ]] && missing+=("-l P_LEAVE")
elif [[ "$ACTION" == "clear" ]]; then
  [[ -z "$PREFIX" ]] && missing+=("-p PREFIX")
fi

if (( ${#missing[@]} > 0 )); then
  echo "[ERROR] Missing required parameters: ${missing[*]}"
  usage
fi


# Helper functions
get_active_containers() {
  docker ps --format '{{.Names}}' | grep "^${PREFIX}" || true
}

get_stopped_containers() {
  docker ps -a --filter "status=exited" --format '{{.Names}}' | grep "^${PREFIX}" || true
}

join_node() {
  local stopped target
  stopped=$(get_stopped_containers)
  if [[ -n "$stopped" ]]; then
    target=$(echo "$stopped" | shuf -n 1)
    echo "[JOIN] Starting $target..."
    docker start "$target" >/dev/null
  else
    echo "[JOIN] No stopped containers available."
  fi
}

leave_node() {
  local active count target
  active=$(get_active_containers)
  count=$(echo "$active" | wc -l || echo 0)
  if (( count > MIN_ACTIVE )); then
    target=$(echo "$active" | shuf -n 1)
    echo "[LEAVE] Stopping $target..."
    docker stop "$target" >/dev/null
  else
    echo "[LEAVE] Skipping — only $count active (min=$MIN_ACTIVE)."
  fi
}

stop_churn() {
  echo
  echo "[INFO] Stopping churn controller gracefully..."
  exit 0
}
trap stop_churn SIGINT SIGTERM

# Apply action
if [[ "$ACTION" == "apply" ]]; then
  echo "------------------------------------------------------------"
  echo "[INFO] Starting churn controller"
  echo "       Prefix:      $PREFIX"
  echo "       Interval:    ${INTERVAL}s"
  echo "       Min active:  $MIN_ACTIVE"
  echo "       P(join):     $P_JOIN"
  echo "       P(leave):    $P_LEAVE"
  echo "       Log file:    $LOG_FILE"
  echo "------------------------------------------------------------"

  while true; do
    sleep "$INTERVAL"
    r=$(awk -v seed=$RANDOM 'BEGIN{srand(seed); print rand()}')
    if (( $(echo "$r < $P_JOIN" | bc -l) )); then
      join_node
    elif (( $(echo "$r < $P_JOIN + $P_LEAVE" | bc -l) )); then
      leave_node
    else
      echo "[IDLE] No churn this interval."
    fi
  done
fi

# Clear action
if [[ "$ACTION" == "clear" ]]; then
  echo "[INFO] Stopping all churn.sh processes..."
  PIDS=$(pgrep -f "[c]hurn.sh apply" || true)
  if [[ -n "$PIDS" ]]; then
    echo "$PIDS" | while read -r pid; do
      echo "[CLEAR] Killing churn process PID=$pid"
      kill "$pid" 2>/dev/null || true
    done
    sleep 2
  else
    echo "[INFO] No active churn.sh processes found."
  fi

  echo "[SUCCESS] Churn controller stopped."
  echo "[INFO] Containers state preserved (no restarts performed)."
  exit 0
fi
