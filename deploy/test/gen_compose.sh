#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# KoordeDHT Docker Compose Generator
# -----------------------------------------------------------------------------
# Generates a docker-compose file replacing template variables for the tester.
#
# Usage:
#   ./gen_compose.sh \
#       --sim-duration <duration> \
#       --query-rate <rate> \
#       --query-parallelism-min <min> \
#       --query-parallelism-max <max>
#
# Example:
#   ./gen_compose.sh --sim-duration 2m --query-rate 0.8 \
#       --query-parallelism-min 2 --query-parallelism-max 10
# -----------------------------------------------------------------------------

LOG_FILE="/var/log/test/gen_compose.log"
TEMPLATE="docker-compose.template.yml"
OUTPUT="docker-compose.generated.yml"

# Redirect stdout/stderr both to console and log
exec > >(tee -a "$LOG_FILE") 2>&1

# --- Usage -------------------------------------------------------------------
usage() {
  echo
  echo "Usage:"
  echo "  $0 --sim-duration <duration> --query-rate <rate> \\"
  echo "     --query-parallelism-min <min> --query-parallelism-max <max> --query-timeout <timeout>"
  echo
  echo "Example:"
  echo "  $0 --sim-duration 1m --query-rate 0.5 --query-parallelism-min 1 --query-parallelism-max 5 --query-timeout 10s"
  echo
  echo "Description:"
  echo "  Generates a docker-compose file replacing placeholders in:"
  echo "    \${SIM_DURATION}, \${QUERY_RATE}, \${QUERY_PARALLELISM_MIN}, \${QUERY_PARALLELISM_MAX}, \${QUERY_TIMEOUT}"
  echo
  exit 1
}

# --- Parse arguments ---------------------------------------------------------
SIM_DURATION=""
QUERY_RATE=""
QUERY_PARALLELISM_MIN=""
QUERY_PARALLELISM_MAX=""
QUERY_TIMEOUT="10s"  # Default timeout for queries

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sim-duration)          SIM_DURATION="$2"; shift 2 ;;
    --query-rate)            QUERY_RATE="$2"; shift 2 ;;
    --query-parallelism-min) QUERY_PARALLELISM_MIN="$2"; shift 2 ;;
    --query-parallelism-max) QUERY_PARALLELISM_MAX="$2"; shift 2 ;;
    --query-timeout)         QUERY_TIMEOUT="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) echo "Error: unknown argument '$1'"; usage ;;
  esac
done

# --- Validate input ----------------------------------------------------------
if [[ -z "$SIM_DURATION" || -z "$QUERY_RATE" || -z "$QUERY_PARALLELISM_MIN" || -z "$QUERY_PARALLELISM_MAX" ]]; then
  echo "Error: all parameters are required."
  usage
fi

if [[ ! -f "$TEMPLATE" ]]; then
  echo "Error: template file $TEMPLATE not found!"
  exit 1
fi

# --- Generate file -----------------------------------------------------------
echo
echo "Generating $OUTPUT ..."
sed \
  -e "s|\${SIM_DURATION}|${SIM_DURATION}|g" \
  -e "s|\${QUERY_RATE}|${QUERY_RATE}|g" \
  -e "s|\${QUERY_PARALLELISM_MIN}|${QUERY_PARALLELISM_MIN}|g" \
  -e "s|\${QUERY_PARALLELISM_MAX}|${QUERY_PARALLELISM_MAX}|g" \
  -e "s|\${QUERY_TIMEOUT}|${QUERY_TIMEOUT}|g" \
  "$TEMPLATE" > "$OUTPUT"

echo
echo "Generated $OUTPUT successfully."
echo "  SIM_DURATION=${SIM_DURATION}"
echo "  QUERY_RATE=${QUERY_RATE}"
echo "  QUERY_PARALLELISM_MIN=${QUERY_PARALLELISM_MIN}"
echo "  QUERY_PARALLELISM_MAX=${QUERY_PARALLELISM_MAX}"
echo "  QUERY_TIMEOUT=${QUERY_TIMEOUT}"
echo
echo "Logs saved to: $LOG_FILE"
exit 0
