#!/bin/bash
# Resets the Skyward Chronicles demo to a clean state
# This is a convenience wrapper around setup-skyward-demo.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Resetting Skyward Chronicles demo..."
echo ""

# Just run the setup script, which handles cleanup
"$SCRIPT_DIR/setup-skyward-demo.sh"
