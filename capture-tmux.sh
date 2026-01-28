#!/bin/bash
# Capture tmux pane screenshots every 0.5 seconds

SESSION_NAME="rela-demo"
OUTPUT_DIR="screendumps"
INTERVAL=0.5

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Counter for sequential numbering
counter=1

echo "Starting screen capture of tmux session '$SESSION_NAME'"
echo "Output directory: $OUTPUT_DIR"
echo "Interval: ${INTERVAL}s"
echo "Press Ctrl+C to stop"
echo ""

# Trap Ctrl+C to clean up
trap 'echo -e "\n\nStopped. Captured $counter screenshots."; exit 0' INT

while true; do
    # Format counter with leading zeros (e.g., 0001, 0002, etc.)
    filename=$(printf "%s/screen_%04d.txt" "$OUTPUT_DIR" "$counter")

    # Capture tmux pane
    if tmux capture-pane -t "$SESSION_NAME" -p > "$filename" 2>/dev/null; then
        echo "Captured: $filename"
        ((counter++))
    else
        echo "Error: Could not capture pane from session '$SESSION_NAME'"
        echo "Make sure the session exists: tmux ls"
        exit 1
    fi

    # Wait before next capture
    sleep "$INTERVAL"
done
