#!/bin/bash
# hack/cleanup.sh - Cleanup agents and test directory

REPO_ROOT=$(pwd)
TEST_DIR="${REPO_ROOT}/../scion-qa-temp"

echo "=== Cleaning up agents ==="

# Stop all agents started by scion
if [ -f "${TEST_DIR}/scion" ]; then
    AGENTS=$("${TEST_DIR}/scion" list | tail -n +2 | awk '{print $1}')
    for agent in $AGENTS; do
        if [ -n "$agent" ]; then
            "${TEST_DIR}/scion" stop "$agent"
        fi
    done
fi

# Fallback for Apple container
# if command -v container &> /dev/null; then
#     IDS=$(container list -a --format json | grep '"scion.agent":"true"' -B 20 | grep '"id":' | cut -d'"' -f4)
#     for id in $IDS; do
#         container stop "$id" || true
#         container rm "$id" || true
#     done
# fi

# Delete contents but keep the directory
echo "=== Cleaning up test directory contents ==="
if [ -d "${TEST_DIR}" ]; then
    find "${TEST_DIR}" -mindepth 1 -delete
fi

echo "=== Cleanup Complete ==="
