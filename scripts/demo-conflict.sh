#!/bin/bash
# Creates a merge conflict scenario for testing conflict resolution
# Both local and remote modify the same file

set -e

DEMO_ROOT="/tmp/rela-demos"
ORIGIN_DIR="$DEMO_ROOT/skyward-origin.git"
PROJECT_DIR="$DEMO_ROOT/skyward-demo"
TEMP_CLONE="$DEMO_ROOT/temp-clone"

if [ ! -d "$ORIGIN_DIR" ]; then
    echo "Error: Demo not set up. Run setup-skyward-demo.sh first."
    exit 1
fi

echo "=== Creating Conflict Scenario ==="
echo ""

# First, make a local change and commit it
cd "$PROJECT_DIR"

echo "Making local changes..."

# Modify CHAR-001 locally
sed -i.bak 's/level: 15/level: 20/' entities/characters/CHAR-001.md
rm entities/characters/CHAR-001.md.bak

git add .
git commit -m "Increase Captain Mira's level to 20" > /dev/null

# Now make a DIFFERENT change to the same file on "remote"
rm -rf "$TEMP_CLONE"
git clone "$ORIGIN_DIR" "$TEMP_CLONE" > /dev/null 2>&1
cd "$TEMP_CLONE"

git config user.email "remote@skyward.local"
git config user.name "Remote User"

echo "Making conflicting remote changes..."

# Make a different change to the same field
sed -i.bak 's/level: 15/level: 25/' entities/characters/CHAR-001.md
# Also modify the description
sed -i.bak 's/Leader of the Sky Merchants Guild/Legendary leader of the Sky Merchants Guild/' entities/characters/CHAR-001.md
rm entities/characters/CHAR-001.md.bak

git add .
git commit -m "Increase Captain Mira's level to 25, update description" > /dev/null
git push origin main > /dev/null 2>&1

# Clean up temp clone
rm -rf "$TEMP_CLONE"

echo ""
echo "Conflict scenario created!"
echo ""
echo "Local change: CHAR-001 level = 20"
echo "Remote change: CHAR-001 level = 25, description updated"
echo ""
echo "To trigger the conflict:"
echo "  1. Click 'Sync' in the UI, or"
echo "  2. Run: cd $PROJECT_DIR && git pull"
echo ""
echo "This will create conflict markers in CHAR-001.md"
echo "Then use the Conflicts view in rela to resolve."
echo ""
