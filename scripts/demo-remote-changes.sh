#!/bin/bash
# Simulates remote changes by pushing directly to origin
# This creates a "remote ahead" scenario for testing sync/pull

set -e

DEMO_ROOT="/tmp/rela-demos"
ORIGIN_DIR="$DEMO_ROOT/skyward-origin.git"
PROJECT_DIR="$DEMO_ROOT/skyward-demo"
TEMP_CLONE="$DEMO_ROOT/temp-clone"

if [ ! -d "$ORIGIN_DIR" ]; then
    echo "Error: Demo not set up. Run setup-skyward-demo.sh first."
    exit 1
fi

echo "=== Simulating Remote Changes ==="
echo ""

# Clone origin to a temp directory
rm -rf "$TEMP_CLONE"
git clone "$ORIGIN_DIR" "$TEMP_CLONE" > /dev/null 2>&1
cd "$TEMP_CLONE"

git config user.email "remote@skyward.local"
git config user.name "Remote User"

# Make some changes that simulate "another user's work"
echo "Creating remote changes..."

# Add a new item
mkdir -p entities/items
cat > entities/items/ITEM-009.md << 'EOF'
---
id: ITEM-009
name: Thunder Crystal
category: material
rarity: rare
value: 300
description: A crystal that hums with electrical energy
---

# Usage

Crafting material for lightning-infused weapons. Found in storm clouds.
EOF

# Add a new relation
mkdir -p relations
cat > "relations/ITEM-009--found-at--LOC-002.md" << 'EOF'
---
from: ITEM-009
type: found-at
to: LOC-002
---
EOF

# Update an existing quest
sed -i.bak 's/xp_reward: 100/xp_reward: 125/' entities/quests/QUEST-001.md
rm entities/quests/QUEST-001.md.bak

git add .
git commit -m "Add Thunder Crystal item; Increase QUEST-001 XP reward" > /dev/null
git push origin main > /dev/null 2>&1

# Clean up temp clone
rm -rf "$TEMP_CLONE"

echo "Remote changes pushed!"
echo ""
echo "Changes made:"
echo "  - Added ITEM-009 (Thunder Crystal)"
echo "  - Added relation: ITEM-009 found-at LOC-002"
echo "  - Updated QUEST-001 XP reward: 100 -> 125"
echo ""
echo "The demo project now has 1 commit behind remote."
echo "Use the Sync button in the UI or run 'git pull' to see these changes."
echo ""
