#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Koorde DHT Churn Controller
# -----------------------------------------------------------------------------
# Randomly stops and starts Docker containers to simulate churn.
# Works only with containers already defined in docker-compose.
#
# Usage:
#   ./churn.sh -p PREFIX -i INTERVAL -m MIN_ACTIVE -j P_JOIN -l P_LEAVE
#
# Example:
#   ./churn.sh -p localtest-node -i 15 -m 3 -j 0.5 -l 0.4
# -----------------------------------------------------------------------------

usage() {
  echo "Usage: $0 [options]"
  echo
  echo "Options:"
  echo "  -p PREFIX       Prefix of container names (e.g. 'node')"
  echo "  -i INTERVAL     Interval in seconds between churn events (default: 10)"
  echo "  -m MIN_ACTIVE   Minimum number of active containers to keep (default: 2)"
  echo "  -j P_JOIN       Probability of performing a join (0.0–1.0, default: 0.5)"
  echo "  -l P_LEAVE      Probability of performing a leave (0.0–1.0, default: 0.5)"
  echo "  -h              Show this help message"
  echo
  echo "Example:"
  echo "  $0 -p node -i 20 -m 3 -j 0.4 -l 0.3"
  exit 0
}

# --- Default values ---
PREFIX=""
INTERVAL=10
MIN_ACTIVE=2
P_JOIN=0.5
P_LEAVE=0.5

# --- Parse options ---
while getopts ":p:i:m:j:l:h" opt; do
  case ${opt} in
    p) PREFIX="${OPTARG}" ;;
    i) INTERVAL="${OPTARG}" ;;
    m) MIN_ACTIVE="${OPTARG}" ;;
    j) P_JOIN="${OPTARG}" ;;
    l) P_LEAVE="${OPTARG}" ;;
    h) usage ;;
    \?) echo "Invalid option: -$OPTARG" >&2; usage ;;
    :) echo "Option -$OPTARG requires an argument." >&2; usage ;;
  esac
done

if [[ -z "$PREFIX" ]]; then
  echo "Error: -p PREFIX is required."
  usage
fi

# --- Helpers ---
get_active_containers() {
  docker ps --format '{{.Names}}' | grep "^${PREFIX}-" || true
}

join_node() {
  local stopped
  stopped=$(docker ps -a --filter "status=exited" --format '{{.Names}}' | grep "^${PREFIX}-" || true)
  if [[ -n "$stopped" ]]; then
    local target
    target=$(echo "$stopped" | shuf -n 1)
    echo -e "\033[0;32m[JOIN]\033[0m Starting $target..."
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
    echo -e "\033[0;31m[LEAVE]\033[0m Stopping $target..."
    docker stop "$target" >/dev/null
  else
    echo "[LEAVE] Skip — only $count active (min=$MIN_ACTIVE)."
  fi
}

# --- Main ---
echo "==> Starting churn controller"
echo "    Prefix:      $PREFIX"
echo "    Interval:    ${INTERVAL}s"
echo "    Min active:  $MIN_ACTIVE"
echo "    P(join):     $P_JOIN"
echo "    P(leave):    $P_LEAVE"
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
