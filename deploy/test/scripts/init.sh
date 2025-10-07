#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# KoordeDHT Full Simulation Orchestrator (Cloud-Aware)
# -----------------------------------------------------------------------------
# If executed on an EC2 instance, downloads required files from an S3 bucket,
# runs the full Koorde simulation (compose + churn + delay),
# waits (SIM_DURATION + 60s), then uploads results back to S3.
# -----------------------------------------------------------------------------
sudo mkdir -p /var/log/test
sudo chmod 777 /var/log/test
LOG_FILE="/var/log/test/init.log"
exec > >(tee -a "$LOG_FILE") 2>&1

GENERATED="docker-compose.generated.yml"

BUCKET=""
DATA_PREFIX=""
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
  echo "  $0 --bucket <s3-bucket> --prefix <folder> --sim-duration <time> \\"
  echo "     --query-rate <rate> --parallel-min <n> --parallel-max <n> \\"
  echo "     --delay <Xms> --jitter <Yms> --loss <Z%> \\"
  echo "     --churn-interval <sec> --churn-min-active <n> \\"
  echo "     --churn-pjoin <p> --churn-pleave <p> --max-nodes <n>"
  echo
  echo "Example:"
  echo "  ./init.sh --bucket koorde-bucket --prefix test \\"
  echo "     --sim-duration 10m --query-rate 0.5 --parallel-min 1 --parallel-max 5 \\"
  echo "     --delay 200ms --jitter 50ms --loss 0.1% \\"
  echo "     --churn-interval 20 --churn-min-active 3 --churn-pjoin 0.4 --churn-pleave 0.3 --max-nodes 10"
  echo
  exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --bucket) BUCKET="$2"; shift 2 ;;
    --prefix) DATA_PREFIX="$2"; shift 2 ;;
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
[[ -z "$BUCKET" ]] && missing+=("--bucket")
[[ -z "$DATA_PREFIX" ]] && missing+=("--prefix")
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

# Detect if running on EC2
IS_EC2=false
if curl -s --connect-timeout 1 http://169.254.169.254/latest/meta-data/instance-id >/dev/null 2>&1; then
  IS_EC2=true
  echo "[INFO] Detected EC2 environment."
else
  echo "[INFO] Running locally (non-EC2)."
fi

# If on EC2, download required files from S3
if $IS_EC2; then
  echo "[INFO] Downloading input files from s3://$BUCKET/$DATA_PREFIX/ ..."
  aws s3 cp "s3://$BUCKET/$DATA_PREFIX/" . --recursive \
    --exclude "*" \
    --include "*.sh" \
    --include "*.yml" \
    --include "*.yaml" \
    --include "*.env"

  echo "[INFO] Setting executable permissions for shell scripts..."
  for f in *.sh; do
    if [[ -f "$f" ]]; then
      chmod +x "$f"
      echo "  [+] $f made executable"
    fi
  done

  # Double-check that key scripts exist and are executable
  for required in init.sh gen_compose.sh churn.sh; do
    if [[ ! -x "$required" ]]; then
      echo "[ERROR] Required script '$required' missing or not executable."
      exit 1
    fi
  done
fi

# check docker dependencies
for cmd in docker docker-compose tc; do
  command -v $cmd >/dev/null 2>&1 || { echo "[ERROR] Missing $cmd command."; exit 1; }
done

# SELinux Handling (temporarily disable for Pumba compatibility)
ORIGINAL_SELINUX_MODE=""

if command -v getenforce >/dev/null 2>&1; then
  ORIGINAL_SELINUX_MODE=$(getenforce || echo "Unknown")
  if [[ "$ORIGINAL_SELINUX_MODE" != "Disabled" && "$ORIGINAL_SELINUX_MODE" != "Permissive" ]]; then
    echo "[INFO] SELinux currently in $ORIGINAL_SELINUX_MODE mode — disabling temporarily..."
    setenforce 0 || echo "[WARN] Failed to set SELinux permissive mode (requires root)."
  else
    echo "[INFO] SELinux already permissive or disabled."
  fi
else
  echo "[INFO] SELinux tools not found — skipping disable step."
fi

# get the project name and set docker suffix
DOCKER_PROJECT=$(basename "$(realpath "$(pwd)")")
DOCKER_SUFFIX_NODE="${DOCKER_PROJECT}-node-"
echo "[INFO] Using Docker project name: ${DOCKER_PROJECT}"


# generate compose
echo "[INFO] Generating Docker Compose file..."
./gen_compose.sh \
  --sim-duration "$SIM_DURATION" \
  --query-rate "$QUERY_RATE" \
  --query-parallelism-min "$PARALLEL_MIN" \
  --query-parallelism-max "$PARALLEL_MAX" \
  --docker-suffix "$DOCKER_SUFFIX_NODE"
echo "[OK] Generated $GENERATED."

# start docker compose
echo "[INFO] Starting Docker Compose with $MAX_NODES nodes..."
docker-compose -f "$GENERATED" up -d --scale node="$MAX_NODES"
echo "[OK] Compose cluster up."

# apply network delay
echo "[INFO] Starting Pumba network emulator..."
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
  -p "$DOCKER_SUFFIX_NODE" \
  -i "$CHURN_INTERVAL" \
  -m "$CHURN_MIN_ACTIVE" \
  -j "$CHURN_PJOIN" \
  -l "$CHURN_PLEAVE" &
CHURN_PID=$!
echo "[OK] Churn controller started with PID $CHURN_PID."

# wait for simulation duration
EXTRA_WAIT=60
# Convert duration (e.g., 5m, 300s, 2h) into seconds
UNIT="${SIM_DURATION: -1}"
VALUE="${SIM_DURATION%?}"

case "$UNIT" in
  s) TOTAL_SECONDS=$((VALUE + EXTRA_WAIT)) ;;
  m) TOTAL_SECONDS=$((VALUE * 60 + EXTRA_WAIT)) ;;
  h) TOTAL_SECONDS=$((VALUE * 3600 + EXTRA_WAIT)) ;;
  *) echo "[WARN] Unknown duration unit '$UNIT', defaulting to seconds."; TOTAL_SECONDS=$((VALUE + EXTRA_WAIT)) ;;
esac

echo "[INFO] Simulation running for ${SIM_DURATION} + ${EXTRA_WAIT}s (≈${TOTAL_SECONDS}s total)..."
sleep "${TOTAL_SECONDS}s"
echo "[INFO] Simulation window complete."

# stop gratefully
echo "[INFO] Stopping churn and Pumba..."
kill "$CHURN_PID" 2>/dev/null || true
docker stop pumba-delay >/dev/null 2>&1 || true
docker-compose -f "$GENERATED" down
echo "[OK] All components stopped."

# If on EC2, upload results to S3
if $IS_EC2; then
  TS=$(date +%Y%m%dT%H%M%S)
  OUTNAME="output_${TS}_delay-${DELAY}_jitter-${JITTER}_churn-${CHURN_INTERVAL}s-${CHURN_PJOIN}-${CHURN_PLEAVE}"
  OUTLOG="${OUTNAME}.log"
  OUTCSV="${OUTNAME}.csv"

  echo "[INFO] Collecting output logs..."
  cp "$LOG_FILE" "./$OUTLOG" || touch "./$OUTLOG"

  # Upload the main log file
  echo "[INFO] Uploading log to s3://$BUCKET/$DATA_PREFIX/results/$OUTLOG ..."
  aws s3 cp "./$OUTLOG" "s3://$BUCKET/$DATA_PREFIX/results/$OUTLOG"
  echo "[OK] Log uploaded."

  # Upload the results CSV if it exists
  RESULT_CSV="./results/output.csv"
  if [[ -f "$RESULT_CSV" ]]; then
    echo "[INFO] Found results file: $RESULT_CSV"
    cp "$RESULT_CSV" "./$OUTCSV"
    echo "[INFO] Uploading CSV to s3://$BUCKET/$DATA_PREFIX/results/$OUTCSV ..."
    aws s3 cp "./$OUTCSV" "s3://$BUCKET/$DATA_PREFIX/results/$OUTCSV"
    echo "[OK] CSV uploaded."
  else
    echo "[WARN] No output CSV found at $RESULT_CSV — skipping upload."
  fi
fi


# Restore SELinux state
if [[ -n "${ORIGINAL_SELINUX_MODE:-}" ]]; then
  if [[ "$ORIGINAL_SELINUX_MODE" == "Enforcing" ]]; then
    echo "[INFO] Restoring SELinux mode to Enforcing..."
    setenforce 1 || echo "[WARN] Could not restore SELinux (requires root)."
  elif [[ "$ORIGINAL_SELINUX_MODE" == "Permissive" ]]; then
    echo "[INFO] Restoring SELinux to Permissive (previous state)."
    setenforce 0 || true
  fi
fi

echo "[SUCCESS] Full simulation cycle completed."
