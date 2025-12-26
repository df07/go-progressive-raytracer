#!/bin/bash
# Helper script for logging documentation access
# Usage: log-doc-access.sh <file_path> <score> <comment>

if [ $# -lt 3 ]; then
    echo "Usage: $0 <file_path> <score> <comment>"
    exit 1
fi

FILE_PATH="$1"
SCORE="$2"
shift 2
COMMENT="$*"

# Get UTC timestamp in ISO 8601 format
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Append access log atomically
echo "${TIMESTAMP} ${SCORE} ${COMMENT}" >> "${FILE_PATH}"
