#!/bin/bash
# Setup script for Skyward Chronicles demo project
# Creates a full rela project with git history in /tmp/rela-demos/

set -e

# Capture script directory BEFORE any cd commands
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

DEMO_ROOT="/tmp/rela-demos"
ORIGIN_DIR="$DEMO_ROOT/skyward-origin.git"
PROJECT_DIR="$DEMO_ROOT/skyward-demo"

echo "=== Skyward Chronicles Demo Setup ==="
echo ""

# Clean up existing
if [ -d "$DEMO_ROOT" ]; then
    echo "Removing existing demo at $DEMO_ROOT..."
    rm -rf "$DEMO_ROOT"
fi

mkdir -p "$DEMO_ROOT"

# Create bare origin repository
echo "Creating origin repository..."
git init --bare "$ORIGIN_DIR" > /dev/null
git -C "$ORIGIN_DIR" symbolic-ref HEAD refs/heads/main

# Clone to working directory
echo "Cloning to working directory..."
git clone "$ORIGIN_DIR" "$PROJECT_DIR" > /dev/null 2>&1
cd "$PROJECT_DIR"

# Configure git for commits
git config user.email "demo@skyward.local"
git config user.name "Skyward Demo"

#############################################################################
# PHASE 1: Project Configuration
#############################################################################
echo "Creating project configuration..."

mkdir -p entities relations .rela .rela/bin

# Copy rela binary for document commands
if [ -f "$SCRIPT_DIR/../bin/rela" ]; then
    cp "$SCRIPT_DIR/../bin/rela" .rela/bin/
fi

# metamodel.yaml
cat > metamodel.yaml << 'METAMODEL_EOF'
version: "1.0"

types:
  character_role:
    values: [npc, enemy, boss, companion]
    default: npc
  location_type:
    values: [city, dungeon, wilderness, landmark, hub]
    default: wilderness
  quest_category:
    values: [main, side, daily, hidden]
    default: side
  quest_status:
    values: [draft, ready, testing, released]
    default: draft
  item_category:
    values: [weapon, armor, consumable, key_item, material]
    default: consumable
  rarity:
    values: [common, uncommon, rare, epic, legendary]
    default: common
  danger_level:
    values: [safe, low, medium, high, extreme]
    default: medium
  ability_type:
    values: [combat, utility, passive, ultimate]
    default: combat
  achievement_category:
    values: [exploration, combat, story, collection, social]
    default: exploration

entities:
  character:
    label: Character
    id_prefix: "CHAR-"
    id_type: sequential
    properties:
      name:
        type: string
        required: true
      role:
        type: character_role
      level:
        type: integer
        default: 1
      essential:
        type: boolean
        default: false
      description:
        type: string

  location:
    label: Location
    id_prefix: "LOC-"
    id_type: sequential
    properties:
      name:
        type: string
        required: true
      category:
        type: location_type
      danger:
        type: danger_level
      description:
        type: string

  quest:
    label: Quest
    id_prefix: "QUEST-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      category:
        type: quest_category
      status:
        type: quest_status
      difficulty:
        type: integer
        default: 1
      xp_reward:
        type: integer
      description:
        type: string

  item:
    label: Item
    id_prefix: "ITEM-"
    id_type: sequential
    properties:
      name:
        type: string
        required: true
      category:
        type: item_category
      rarity:
        type: rarity
      value:
        type: integer
      description:
        type: string

  ability:
    label: Ability
    id_prefix: "ABL-"
    id_type: sequential
    properties:
      name:
        type: string
        required: true
      category:
        type: ability_type
      cooldown:
        type: integer
      description:
        type: string

  lore:
    label: Lore Entry
    id_prefix: "LORE-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      era:
        type: string
      description:
        type: string

  achievement:
    label: Achievement
    id_prefix: "ACH-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      category:
        type: achievement_category
      points:
        type: integer
        default: 10
      hidden:
        type: boolean
        default: false
      description:
        type: string

relations:
  found-at:
    label: Found At
    from: [character, item]
    to: [location]
  rewards:
    label: Rewards
    from: [quest]
    to: [item]
  requires:
    label: Requires
    from: [quest, ability]
    to: [item, quest, ability]
  unlocks:
    label: Unlocks
    from: [quest, achievement]
    to: [location, ability, item]
  teaches:
    label: Teaches
    from: [character]
    to: [ability]
  gives-quest:
    label: Gives Quest
    from: [character]
    to: [quest]
  mentions:
    label: Mentions
    from: [lore]
    to: [character, location, item]
  triggers:
    label: Triggers
    from: [quest]
    to: [achievement]
  takes-place-in:
    label: Takes Place In
    from: [quest]
    to: [location]
  sells:
    label: Sells
    from: [character]
    to: [item]
  drops:
    label: Drops
    from: [character]
    to: [item]
  guards:
    label: Guards
    from: [character]
    to: [location, item]
METAMODEL_EOF

# data-entry.yaml
cat > data-entry.yaml << 'DATAENTRY_EOF'
app:
  title: Skyward Chronicles
  subtitle: Game Design Document
  version: "0.1.0"

git:
  enabled: true
  mode: direct
  branch: main

styles:
  quest_status:
    draft: gray
    ready: blue
    testing: purple
    released: green
  rarity:
    common: gray
    uncommon: green
    rare: blue
    epic: purple
    legendary: orange
  danger_level:
    safe: green
    low: blue
    medium: yellow
    high: orange
    extreme: red

# Document rendering - generates formatted documents from entity data
# NOTE: Uses inline piping (rela show | jq) instead of storing in a variable
# because echoing JSON from a shell variable corrupts control characters
documents:
  character_sheet:
    title: "Character Sheet"
    view: character-context
    command: |
      export PATH=".rela/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:$PATH"
      NAME=$(rela show {id} -o json | jq -r '.entity.properties.name // "Unknown"')
      ROLE=$(rela show {id} -o json | jq -r '.entity.properties.role // "npc"')
      LEVEL=$(rela show {id} -o json | jq -r '.entity.properties.level // 1')
      ESSENTIAL=$(rela show {id} -o json | jq -r '.entity.properties.essential // false')
      DESC=$(rela show {id} -o json | jq -r '.entity.properties.description // ""')
      BODY=$(rela show {id} -o json | jq -r '.entity.content // ""')
      echo "# $NAME"
      echo ""
      echo "**Role:** $ROLE | **Level:** $LEVEL | **Essential:** $ESSENTIAL"
      echo ""
      echo "---"
      echo ""
      echo "## Description"
      echo ""
      echo "$DESC"
      echo ""
      echo "## Background"
      echo ""
      echo "$BODY"
      echo ""
      echo "---"
      echo "*Generated from Skyward Chronicles GDD*"
    timeout: 30

  quest_briefing:
    title: "Quest Briefing"
    view: quest-full-context
    command: |
      export PATH=".rela/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:$PATH"
      TITLE=$(rela show {id} -o json | jq -r '.entity.properties.title // "Untitled Quest"')
      CATEGORY=$(rela show {id} -o json | jq -r '.entity.properties.category // "side"')
      STATUS=$(rela show {id} -o json | jq -r '.entity.properties.status // "draft"')
      DIFFICULTY=$(rela show {id} -o json | jq -r '.entity.properties.difficulty // 1')
      XP=$(rela show {id} -o json | jq -r '.entity.properties.xp_reward // 0')
      GOLD=$(rela show {id} -o json | jq -r '.entity.properties.gold_reward // 0')
      BODY=$(rela show {id} -o json | jq -r '.entity.content // ""')
      echo "# $TITLE"
      echo ""
      echo "**Category:** $CATEGORY | **Difficulty:** $DIFFICULTY/5 | **Status:** $STATUS"
      echo ""
      echo "---"
      echo ""
      echo "## Quest Details"
      echo ""
      echo "$BODY"
      echo ""
      echo "---"
      echo ""
      echo "## Rewards"
      echo ""
      echo "| Type | Amount |"
      echo "|------|--------|"
      echo "| XP   | $XP   |"
      echo "| Gold | $GOLD |"
      echo ""
      echo "---"
      echo "*Quest Briefing - Skyward Chronicles*"
    timeout: 30

forms:
  create_character:
    title: New Character
    entity_type: character
    fields:
      - property: name
      - property: role
      - property: level
      - property: essential
      - property: description

  edit_character:
    title: Edit Character
    entity_type: character
    mode: edit
    body: true
    fields:
      - property: name
      - property: role
      - property: level
      - property: essential
    relations:
      - relation: found-at
      - relation: gives-quest
        widget: multi-select
      - relation: teaches
        widget: multi-select
      - relation: sells
        widget: multi-select
      - relation: drops
        widget: multi-select
    side_panel:
      traverse:
        - from: entry
          follow: found-at
          collect_as: location
        - from: entry
          follow: gives-quest
          collect_as: quests
        - from: entry
          follow: teaches
          collect_as: abilities
        - from: entry
          follow: sells
          collect_as: shop_items
        - from: entry
          follow: drops
          collect_as: drop_items
      sections:
        - heading: "Location"
          source: location
          display: cards
          fields:
            - property: name
            - property: category
            - property: danger
          empty_message: "No location set"
        - heading: "Quests Given"
          source: quests
          display: list
          fields:
            - property: title
            - property: status
          empty_message: "Gives no quests"
        - heading: "Teaches"
          source: abilities
          display: list
          fields:
            - property: name
            - property: category
          empty_message: "Teaches no abilities"
        - heading: "Stats"
          source: entry
          display: properties
          fields:
            - property: level
            - property: role
            - property: essential

  create_location:
    title: New Location
    entity_type: location
    fields:
      - property: name
      - property: category
      - property: danger
      - property: description

  edit_location:
    title: Edit Location
    entity_type: location
    mode: edit
    body: true
    fields:
      - property: name
      - property: category
      - property: danger
    side_panel:
      traverse:
        - from: entry
          follow_incoming: found-at
          collect_as: characters
        - from: entry
          follow_incoming: takes-place-in
          collect_as: quests
        - from: entry
          follow_incoming: guards
          collect_as: guardians
      sections:
        - heading: "Characters Here"
          source: characters
          display: cards
          fields:
            - property: name
            - property: role
            - property: level
          empty_message: "No characters at this location"
        - heading: "Quests Here"
          source: quests
          display: list
          fields:
            - property: title
            - property: status
          empty_message: "No quests at this location"
        - heading: "Guarded By"
          source: guardians
          display: list
          fields:
            - property: name
            - property: level
          empty_message: "Unguarded"

  create_quest:
    title: New Quest
    entity_type: quest
    fields:
      - property: title
      - property: category
      - property: status
      - property: difficulty
      - property: xp_reward
      - property: description

  edit_quest:
    title: Edit Quest
    entity_type: quest
    mode: edit
    body: true
    fields:
      - property: title
      - property: category
      - property: status
      - property: difficulty
      - property: xp_reward
    relations:
      - relation: takes-place-in
      - relation: rewards
        widget: multi-select
      - relation: requires
        widget: multi-select
      - relation: unlocks
        widget: multi-select
      - relation: triggers
        widget: multi-select
    side_panel:
      traverse:
        - from: entry
          follow_incoming: gives-quest
          collect_as: quest_giver
        - from: entry
          follow: takes-place-in
          collect_as: location
        - from: entry
          follow: rewards
          collect_as: rewards
        - from: entry
          follow: requires
          collect_as: requirements
        - from: entry
          follow: unlocks
          collect_as: unlocks
      sections:
        - heading: "Quest Giver"
          source: quest_giver
          display: cards
          fields:
            - property: name
            - property: role
          empty_message: "No quest giver"
        - heading: "Location"
          source: location
          display: cards
          fields:
            - property: name
            - property: danger
          empty_message: "No location"
        - heading: "Rewards"
          source: rewards
          display: list
          fields:
            - property: name
            - property: rarity
          empty_message: "No item rewards"
        - heading: "Requirements"
          source: requirements
          display: list
          fields:
            - property: name
          empty_message: "No requirements"
        - heading: "Quest Info"
          source: entry
          display: properties
          fields:
            - property: status
            - property: difficulty
            - property: xp_reward

  create_item:
    title: New Item
    entity_type: item
    fields:
      - property: name
      - property: category
      - property: rarity
      - property: value
      - property: description

  edit_item:
    title: Edit Item
    entity_type: item
    mode: edit
    body: true
    fields:
      - property: name
      - property: category
      - property: rarity
      - property: value
    relations:
      - relation: found-at
    side_panel:
      traverse:
        - from: entry
          follow_incoming: rewards
          collect_as: quest_rewards
        - from: entry
          follow_incoming: sells
          collect_as: vendors
        - from: entry
          follow_incoming: drops
          collect_as: drop_sources
        - from: entry
          follow: found-at
          collect_as: locations
      sections:
        - heading: "Quest Rewards"
          source: quest_rewards
          display: list
          fields:
            - property: title
          empty_message: "Not a quest reward"
        - heading: "Sold By"
          source: vendors
          display: list
          fields:
            - property: name
          empty_message: "Not sold by anyone"
        - heading: "Dropped By"
          source: drop_sources
          display: list
          fields:
            - property: name
            - property: level
          empty_message: "Not dropped by anyone"
        - heading: "Found At"
          source: locations
          display: list
          fields:
            - property: name
          empty_message: "No fixed location"

  create_ability:
    title: New Ability
    entity_type: ability
    fields:
      - property: name
      - property: category
      - property: cooldown
      - property: description

  create_lore:
    title: New Lore Entry
    entity_type: lore
    fields:
      - property: title
      - property: era
      - property: description

  create_achievement:
    title: New Achievement
    entity_type: achievement
    fields:
      - property: title
      - property: category
      - property: points
      - property: hidden
      - property: description

lists:
  all_characters:
    title: Characters
    entity_type: character
    columns:
      - property: name
        link: document/character_sheet
      - property: role
      - property: level
    sort_by: name
    create_form: create_character
    edit_form: edit_character
    detail_view: character_report

  all_locations:
    title: Locations
    entity_type: location
    columns:
      - property: name
        link: detail
      - property: category
      - property: danger
    sort_by: name
    create_form: create_location
    edit_form: edit_location
    detail_view: location_report

  all_quests:
    title: Quests
    entity_type: quest
    columns:
      - property: title
        link: document/quest_briefing
      - property: category
      - property: status
      - property: difficulty
    sort_by: title
    create_form: create_quest
    edit_form: edit_quest
    detail_view: quest_report

  all_items:
    title: Items
    entity_type: item
    columns:
      - property: name
        link: detail
      - property: category
      - property: rarity
    sort_by: name
    create_form: create_item
    edit_form: edit_item
    detail_view: item_report

  all_abilities:
    title: Abilities
    entity_type: ability
    columns:
      - property: name
      - property: category
      - property: cooldown
    sort_by: name
    create_form: create_ability

  all_lore:
    title: Lore
    entity_type: lore
    columns:
      - property: title
      - property: era
    sort_by: title
    create_form: create_lore

  all_achievements:
    title: Achievements
    entity_type: achievement
    columns:
      - property: title
      - property: category
      - property: points
    sort_by: title
    create_form: create_achievement

kanbans:
  quest_board:
    title: Quest Development
    entity_type: quest
    column_property: status
    card_title: title
    card_subtitle: category
    columns:
      - value: draft
        label: Draft
        color: gray
      - value: ready
        label: Ready
        color: blue
      - value: testing
        label: Testing
        color: yellow
      - value: released
        label: Released
        color: green

views:
  character_report:
    title: "Character Sheet"
    entry:
      type: character
    traverse:
      - from: entry
        follow: found-at
        collect_as: location
      - from: entry
        follow: gives-quest
        collect_as: quests
      - from: entry
        follow: teaches
        collect_as: abilities
      - from: entry
        follow: sells
        collect_as: shop_items
      - from: entry
        follow: drops
        collect_as: drops
    sections:
      - heading: "Character"
        source: entry
        display: properties
        fields:
          - property: name
          - property: role
          - property: level
          - property: essential
      - source: entry
        display: content
      - heading: "Location"
        source: location
        display: cards
        fields:
          - property: name
          - property: category
          - property: danger
        empty_message: "No fixed location"
      - heading: "Quests Given"
        source: quests
        display: table
        columns:
          - property: title
            link: detail
          - property: status
          - property: difficulty
        empty_message: "This character gives no quests"
      - heading: "Abilities Taught"
        source: abilities
        display: list
        fields:
          - property: name
          - property: category
        empty_message: "Teaches no abilities"
      - heading: "Shop Items"
        source: shop_items
        display: table
        columns:
          - property: name
            link: detail
          - property: rarity
          - property: value
        empty_message: "Not a merchant"
      - heading: "Drops"
        source: drops
        display: list
        fields:
          - property: name
          - property: rarity
        empty_message: "No item drops"

  quest_report:
    title: "Quest Details"
    entry:
      type: quest
    traverse:
      - from: entry
        follow_incoming: gives-quest
        collect_as: quest_giver
      - from: entry
        follow: takes-place-in
        collect_as: location
      - from: entry
        follow: rewards
        collect_as: rewards
      - from: entry
        follow: requires
        collect_as: requirements
      - from: entry
        follow: unlocks
        collect_as: unlocks
      - from: entry
        follow: triggers
        collect_as: achievements
    sections:
      - heading: "Quest"
        source: entry
        display: properties
        fields:
          - property: title
          - property: category
          - property: status
          - property: difficulty
          - property: xp_reward
      - source: entry
        display: content
      - heading: "Quest Giver"
        source: quest_giver
        display: cards
        fields:
          - property: name
          - property: role
          - property: level
        empty_message: "Unknown quest giver"
      - heading: "Location"
        source: location
        display: cards
        fields:
          - property: name
          - property: category
          - property: danger
        empty_message: "No specific location"
      - heading: "Rewards"
        source: rewards
        display: table
        columns:
          - property: name
            link: detail
          - property: category
          - property: rarity
        empty_message: "No item rewards"
      - heading: "Requirements"
        source: requirements
        display: list
        fields:
          - property: name
        empty_message: "No requirements"
      - heading: "Unlocks"
        source: unlocks
        display: list
        fields:
          - property: name
        empty_message: "Unlocks nothing"
      - heading: "Achievements"
        source: achievements
        display: list
        fields:
          - property: title
          - property: points
        empty_message: "No associated achievements"

  location_report:
    title: "Location Guide"
    entry:
      type: location
    traverse:
      - from: entry
        follow_incoming: found-at
        collect_as: characters
      - from: entry
        follow_incoming: takes-place-in
        collect_as: quests
      - from: entry
        follow_incoming: guards
        collect_as: guardians
    sections:
      - heading: "Location"
        source: entry
        display: properties
        fields:
          - property: name
          - property: category
          - property: danger
      - source: entry
        display: content
      - heading: "Guardians"
        source: guardians
        display: cards
        fields:
          - property: name
          - property: role
          - property: level
        empty_message: "No guardians"
      - heading: "Characters"
        source: characters
        display: table
        columns:
          - property: name
            link: detail
          - property: role
          - property: level
        empty_message: "No characters at this location"
      - heading: "Quests"
        source: quests
        display: table
        columns:
          - property: title
            link: detail
          - property: status
          - property: difficulty
        empty_message: "No quests at this location"

  item_report:
    title: "Item Details"
    entry:
      type: item
    traverse:
      - from: entry
        follow_incoming: rewards
        collect_as: quest_rewards
      - from: entry
        follow_incoming: sells
        collect_as: vendors
      - from: entry
        follow_incoming: drops
        collect_as: drop_sources
      - from: entry
        follow: found-at
        collect_as: locations
    sections:
      - heading: "Item"
        source: entry
        display: properties
        fields:
          - property: name
          - property: category
          - property: rarity
          - property: value
      - source: entry
        display: content
      - heading: "Quest Rewards"
        source: quest_rewards
        display: list
        fields:
          - property: title
        empty_message: "Not a quest reward"
      - heading: "Sold By"
        source: vendors
        display: list
        fields:
          - property: name
        empty_message: "Not sold by anyone"
      - heading: "Dropped By"
        source: drop_sources
        display: table
        columns:
          - property: name
            link: detail
          - property: role
          - property: level
        empty_message: "Not dropped by any enemy"
      - heading: "Found At"
        source: locations
        display: list
        fields:
          - property: name
        empty_message: "No fixed location"

commands:
  export-json:
    label: "Export JSON"
    script: |
      echo '::rela::{"type":"message","text":"Exporting entity to JSON..."}'
      cat
      echo '::rela::{"type":"message","text":"Done!"}'
    context: entity
    available_on:
      entity_types: [character, quest, item, location]

  generate-character-sheet:
    label: "Generate Character Sheet"
    script: |
      echo '::rela::{"type":"message","text":"Generating character sheet..."}'
      PDF="/tmp/skyward-char-${RELA_ENTITY_ID:-unknown}.pdf"
      printf '%%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj\n4 0 obj<</Length 60>>stream\nBT /F1 24 Tf 100 700 Td (Skyward Chronicles - Character Sheet) Tj ET\nendstream\nendobj\n5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj\nxref\n0 6\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000266 00000 n \n0000000376 00000 n \ntrailer<</Size 6/Root 1 0 R>>\nstartxref\n446\n%%%%EOF\n' > "$PDF"
      echo "::rela::{\"type\":\"message\",\"text\":\"PDF saved to $PDF\"}"
      echo "::rela::{\"type\":\"file\",\"path\":\"$PDF\",\"label\":\"Character Sheet PDF\",\"action\":\"open\"}"
    context: entity
    available_on:
      entity_types: [character]

  generate-quest-briefing:
    label: "Generate Quest Briefing"
    script: |
      echo '::rela::{"type":"message","text":"Generating quest briefing..."}'
      PDF="/tmp/skyward-quest-${RELA_ENTITY_ID:-unknown}.pdf"
      printf '%%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj\n4 0 obj<</Length 55>>stream\nBT /F1 24 Tf 100 700 Td (Skyward Chronicles - Quest Briefing) Tj ET\nendstream\nendobj\n5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj\nxref\n0 6\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000266 00000 n \n0000000371 00000 n \ntrailer<</Size 6/Root 1 0 R>>\nstartxref\n441\n%%%%EOF\n' > "$PDF"
      echo "::rela::{\"type\":\"message\",\"text\":\"PDF saved to $PDF\"}"
      echo "::rela::{\"type\":\"file\",\"path\":\"$PDF\",\"label\":\"Quest Briefing PDF\",\"action\":\"open\"}"
    context: entity
    available_on:
      entity_types: [quest]

  view-item-sources:
    label: "View Item Sources"
    script: |
      echo "::rela::{\"type\":\"message\",\"text\":\"Item: $RELA_ENTITY_ID\"}"
      echo "::rela::{\"type\":\"message\",\"text\":\"Showing all sources for this item...\"}"
    context: view
    available_on:
      views: [item_report]

  project-stats:
    label: "Project Statistics"
    script: |
      echo "::rela::{\"type\":\"message\",\"text\":\"Skyward Chronicles GDD Statistics\"}"
      echo "::rela::{\"type\":\"message\",\"text\":\"Project root: $RELA_PROJECT_ROOT\"}"
    context: global
    available_on:
      dashboard: true

dashboard:
  title: Skyward Chronicles Overview
  widgets:
    - type: count
      title: Characters
      entity_type: character
      icon: users
    - type: count
      title: Locations
      entity_type: location
      icon: map
    - type: count
      title: Quests
      entity_type: quest
      icon: scroll
    - type: count
      title: Items
      entity_type: item
      icon: gem
    - type: recent
      title: Recent Quests
      entity_type: quest
      display_property: title
      limit: 10
    - type: recent
      title: Recent Characters
      entity_type: character
      display_property: name
      limit: 5

navigation:
  - label: Dashboard
    icon: home
    page: dashboard
  - label: Characters
    icon: users
    page: list
    list: all_characters
  - label: Locations
    icon: map
    page: list
    list: all_locations
  - label: Quests
    icon: scroll
    page: list
    list: all_quests
  - label: Quest Board
    icon: columns
    page: kanban
    kanban: quest_board
  - label: Items
    icon: gem
    page: list
    list: all_items
  - label: Abilities
    icon: zap
    page: list
    list: all_abilities
  - label: Lore
    icon: book
    page: list
    list: all_lore
  - label: Achievements
    icon: trophy
    page: list
    list: all_achievements
DATAENTRY_EOF

# views.yaml
cat > views.yaml << 'VIEWS_EOF'
views:
  character-context:
    description: Full character context including location, quests, and items
    entry:
      type: character
    traverse:
      - from: entry
        follow: found-at
        collect_as: locations
      - from: entry
        follow: gives-quest
        collect_as: quests
      - from: entry
        follow: teaches
        collect_as: abilities
      - from: entry
        follow: sells
        collect_as: items_sold
      - from: entry
        follow: drops
        collect_as: drops

  quest-full-context:
    description: Quest with all related entities
    entry:
      type: quest
    traverse:
      - from: entry
        follow: takes-place-in
        collect_as: locations
      - from: entry
        follow_incoming: gives-quest
        collect_as: quest_givers
      - from: entry
        follow: rewards
        collect_as: rewards
      - from: entry
        follow: requires
        collect_as: requirements
        recursive: true
        max_depth: 2
      - from: entry
        follow: unlocks
        collect_as: unlocks
      - from: entry
        follow: triggers
        collect_as: triggers

  location-ecosystem:
    description: Everything at a location
    entry:
      type: location
    traverse:
      - from: entry
        follow_incoming: found-at
        collect_as: characters
      - from: entry
        follow_incoming: takes-place-in
        collect_as: quests
      - from: entry
        follow_incoming: guards
        collect_as: guardians

  item-sources:
    description: Where to get an item
    entry:
      type: item
    traverse:
      - from: entry
        follow: found-at
        collect_as: locations
      - from: entry
        follow_incoming: rewards
        collect_as: quest_rewards
      - from: entry
        follow_incoming: sells
        collect_as: vendors
      - from: entry
        follow_incoming: drops
        collect_as: dropped_by
VIEWS_EOF

git add .
git commit -m "Initial project setup" > /dev/null
git branch -M main

#############################################################################
# PHASE 2: Characters
#############################################################################
echo "Creating characters..."

mkdir -p entities/characters

cat > entities/characters/CHAR-001.md << 'EOF'
---
id: CHAR-001
name: Captain Mira Stormwind
role: npc
level: 15
essential: true
description: Leader of the Sky Merchants Guild
---

# Background

Captain Mira is a veteran airship pilot who now leads the Sky Merchants Guild from Windhollow. She's known for her sharp wit and fair dealings.

# Role in Story

- Main quest giver for Act 1
- Provides the player's first airship
- Has connections throughout the floating islands
EOF

cat > entities/characters/CHAR-002.md << 'EOF'
---
id: CHAR-002
name: Rusty Cogsworth
role: npc
level: 8
essential: false
description: Eccentric inventor and shopkeeper
---

# Background

Rusty runs the Brass & Steam shop in Windhollow. His inventions are unpredictable but occasionally brilliant.

# Services

- Sells mechanical gadgets
- Can upgrade equipment
- Offers side quests related to finding rare parts
EOF

cat > entities/characters/CHAR-003.md << 'EOF'
---
id: CHAR-003
name: Admiral Korrath
role: boss
level: 30
essential: true
description: Commander of the Iron Fleet
---

# Background

Once a decorated naval officer, Korrath seeks to unite the islands under his iron rule. He commands the feared Iron Fleet from his flagship.

# Combat Notes

- Final boss of Act 2
- Three-phase fight
- Vulnerable to lightning damage
EOF

cat > entities/characters/CHAR-004.md << 'EOF'
---
id: CHAR-004
name: Zephyr
role: companion
level: 12
essential: true
description: Wind mage and potential party member
---

# Background

A young wind mage from the Cloudkeeper Monastery. Zephyr joins the party after completing the Temple of Winds quest.

# Abilities

- Wind Blast (combat)
- Tailwind (utility - increases party speed)
- Cyclone Shield (defensive)
EOF

cat > entities/characters/CHAR-005.md << 'EOF'
---
id: CHAR-005
name: Shadow Wyrm
role: boss
level: 25
essential: false
description: Ancient creature guarding the Abyss entrance
---

# Background

A massive serpentine creature corrupted by dark energy. Guards the entrance to The Abyss.

# Combat Notes

- Optional boss
- Drops legendary materials
- Weak to light magic
EOF

cat > entities/characters/CHAR-006.md << 'EOF'
---
id: CHAR-006
name: Elder Nimbus
role: npc
level: 50
essential: true
description: Leader of the Cloudkeeper Monks
---

# Background

The oldest and wisest of the Cloudkeepers. Holds knowledge of the ancient skyward civilization.

# Role

- Provides crucial lore exposition
- Teaches advanced wind abilities
- Keeper of the Heart of Storms artifact
EOF

cat > entities/characters/CHAR-007.md << 'EOF'
---
id: CHAR-007
name: Sky Raider Captain
role: enemy
level: 10
essential: false
description: Generic enemy captain
---

# Combat Role

Mid-tier enemy encountered in the Shattered Isles. Commands groups of raiders.

# Drops

- Gold (50-100)
- Raider equipment (common)
- Rare: Sky Raider's Cutlass
EOF

cat > entities/characters/CHAR-008.md << 'EOF'
---
id: CHAR-008
name: The Collector
role: npc
level: 1
essential: false
description: Mysterious figure who trades rare items
---

# Background

A hooded figure who appears in various locations. Interested in rare artifacts and willing to trade legendary items.

# Services

- Trades rare items for collections
- Accepts achievement tokens
- Sells unique cosmetics
EOF

git add entities/characters/
git commit -m "Add characters" > /dev/null

#############################################################################
# PHASE 3: Locations
#############################################################################
echo "Creating locations..."

mkdir -p entities/locations

cat > entities/locations/LOC-001.md << 'EOF'
---
id: LOC-001
name: Windhollow
category: hub
danger: safe
description: The main hub city floating among the clouds
---

# Overview

Windhollow is the central hub of the game. A bustling city built on a massive floating island, it serves as the player's home base.

# Notable Areas

- Sky Merchants Guild Hall
- Brass & Steam Workshop
- The Cloudy Tankard (tavern)
- Airship Docks
EOF

cat > entities/locations/LOC-002.md << 'EOF'
---
id: LOC-002
name: Shattered Isles
category: wilderness
danger: medium
description: Dangerous archipelago of broken floating rocks
---

# Overview

A treacherous region of unstable floating rocks. Home to sky raiders and ancient ruins.

# Features

- Random encounters with raiders
- Hidden treasure caches
- Ancient artifact sites
EOF

cat > entities/locations/LOC-003.md << 'EOF'
---
id: LOC-003
name: Temple of Winds
category: dungeon
danger: high
description: Ancient monastery of the Cloudkeeper monks
---

# Overview

A sacred place where wind magic originated. Now serves as the Cloudkeeper Monastery.

# Trials

- Trial of Breath (agility puzzle)
- Trial of Gale (combat challenge)
- Trial of Calm (meditation mini-game)
EOF

cat > entities/locations/LOC-004.md << 'EOF'
---
id: LOC-004
name: Ironclad Fortress
category: dungeon
danger: extreme
description: Admiral Korrath's flying fortress
---

# Overview

A massive militarized airship serving as Korrath's mobile base of operations. The setting for Act 2's climax.

# Sections

- Outer Deck (infiltration)
- Engine Room (sabotage objective)
- Command Bridge (boss arena)
EOF

cat > entities/locations/LOC-005.md << 'EOF'
---
id: LOC-005
name: The Abyss
category: dungeon
danger: extreme
description: Mysterious void beneath the clouds
---

# Overview

A realm of darkness beneath the floating islands. Said to be where the world's corruption originates.

# Features

- No natural light
- Requires special equipment to navigate
- Home to the final boss
EOF

git add entities/locations/
git commit -m "Add locations" > /dev/null

#############################################################################
# PHASE 4: Quests
#############################################################################
echo "Creating quests..."

mkdir -p entities/quests

cat > entities/quests/QUEST-001.md << 'EOF'
---
id: QUEST-001
title: A Storm on the Horizon
category: main
status: ready
difficulty: 1
xp_reward: 100
description: The opening quest that introduces the world and main conflict
---

# Objectives

1. Speak with Captain Mira at the Sky Merchants Guild
2. Learn the basic controls (tutorial)
3. Defend Windhollow from raider scouts
4. Report back to Captain Mira

# Notes

This is the tutorial quest. Keep it simple but engaging.
EOF

cat > entities/quests/QUEST-002.md << 'EOF'
---
id: QUEST-002
title: Echoes of the Past
category: main
status: ready
difficulty: 2
xp_reward: 250
description: Explore the Shattered Isles and discover clues about the Iron Fleet
---

# Objectives

1. Travel to the Shattered Isles
2. Investigate the ancient ruins
3. Find evidence of the Iron Fleet's presence
4. Defeat the Sky Raiders guarding the site
5. Return to Captain Mira with your findings

# Notes

The player will encounter their first combat here and discover a mysterious artifact that hints at the larger plot.
EOF

cat > entities/quests/QUEST-003.md << 'EOF'
---
id: QUEST-003
title: Wisdom of the Winds
category: main
status: draft
difficulty: 3
xp_reward: 400
description: Seek the Cloudkeeper Monks to learn about the artifact
---

# Objectives

1. Travel to the Temple of Winds
2. Gain an audience with Elder Nimbus
3. Learn the history of the Heart of Darkness
4. Complete the Trial of Winds
5. Recruit Zephyr as a companion

# Notes

This quest introduces wind magic mechanics and the companion system.
EOF

cat > entities/quests/QUEST-004.md << 'EOF'
---
id: QUEST-004
title: Into the Storm
category: main
status: draft
difficulty: 4
xp_reward: 600
description: Infiltrate the Ironclad Fortress and confront Admiral Korrath
---

# Objectives

1. Find a way to reach the Ironclad Fortress
2. Infiltrate the ship
3. Free the prisoners
4. Confront Admiral Korrath
5. Escape before the fortress self-destructs

# Notes

Major boss battle. Korrath escapes but is wounded, setting up the final confrontation.
EOF

cat > entities/quests/QUEST-005.md << 'EOF'
---
id: QUEST-005
title: The Heart of Darkness
category: main
status: draft
difficulty: 5
xp_reward: 1000
description: Descend into The Abyss and stop Korrath from claiming ultimate power
---

# Objectives

1. Obtain the Abyss Key from the Shadow Wyrm
2. Navigate the treacherous depths
3. Reach the Heart of Darkness
4. Final battle with corrupted Korrath
5. Make a choice that determines the ending

# Notes

Final quest of the main storyline. Multiple endings based on player choices.
EOF

cat > entities/quests/QUEST-006.md << 'EOF'
---
id: QUEST-006
title: Rusty's Heirloom
category: side
status: ready
difficulty: 2
xp_reward: 150
description: Help Rusty recover a family heirloom from the Shattered Isles
---

# Objectives

1. Speak with Rusty about his lost heirloom
2. Search the Shattered Isles for the crashed ship
3. Retrieve the pocket watch
4. Return to Rusty

# Rewards

- Gold
- Discount at Rusty's shop
- Rusty's Old Map (reveals hidden locations)
EOF

git add entities/quests/
git commit -m "Add quests" > /dev/null

#############################################################################
# PHASE 5: Items
#############################################################################
echo "Creating items..."

mkdir -p entities/items

cat > entities/items/ITEM-001.md << 'EOF'
---
id: ITEM-001
name: Sky Captain's Compass
category: key_item
rarity: rare
value: 0
description: A compass that always points to the nearest floating island
---

# Usage

Essential navigation tool. Received from Captain Mira at the start of the game.
EOF

cat > entities/items/ITEM-002.md << 'EOF'
---
id: ITEM-002
name: Windcutter Blade
category: weapon
rarity: uncommon
value: 500
description: A sword infused with wind magic
---

# Stats

- Damage: 25-35
- Speed: Fast
- Special: Attacks create small gusts that can push enemies back
EOF

cat > entities/items/ITEM-003.md << 'EOF'
---
id: ITEM-003
name: Cloudweave Cloak
category: armor
rarity: rare
value: 750
description: A flowing cloak that slows falls
---

# Stats

- Defense: 15
- Special: Reduces fall damage by 50%
- Special: Increases jump height slightly
EOF

cat > entities/items/ITEM-004.md << 'EOF'
---
id: ITEM-004
name: Health Elixir
category: consumable
rarity: common
value: 50
description: Restores health when consumed
---

# Effect

Restores 100 HP instantly. Common drop from most enemies.
EOF

cat > entities/items/ITEM-005.md << 'EOF'
---
id: ITEM-005
name: Abyss Key
category: key_item
rarity: legendary
value: 0
description: A crystallized fragment of darkness that opens the way to The Abyss
---

# Acquisition

Dropped by the Shadow Wyrm upon defeat. Required to access the final dungeon.
EOF

cat > entities/items/ITEM-006.md << 'EOF'
---
id: ITEM-006
name: Heart of Storms
category: key_item
rarity: legendary
value: 0
description: An ancient artifact containing immense wind magic
---

# Lore

One of five elemental hearts created by the ancient skyward civilization. Central to the main plot.
EOF

cat > entities/items/ITEM-007.md << 'EOF'
---
id: ITEM-007
name: Rusty's Pocket Watch
category: key_item
rarity: uncommon
value: 0
description: A worn pocket watch with sentimental value
---

# Notes

Quest item for "Rusty's Heirloom" side quest. Has an inscription inside.
EOF

cat > entities/items/ITEM-008.md << 'EOF'
---
id: ITEM-008
name: Sky Raider's Cutlass
category: weapon
rarity: common
value: 150
description: Standard weapon of sky raiders
---

# Stats

- Damage: 15-20
- Speed: Medium
- Common drop from Sky Raider enemies
EOF

git add entities/items/
git commit -m "Add items" > /dev/null

#############################################################################
# PHASE 6: Abilities
#############################################################################
echo "Creating abilities..."

mkdir -p entities/abilities

cat > entities/abilities/ABL-001.md << 'EOF'
---
id: ABL-001
name: Wind Slash
category: combat
cooldown: 5
description: A blade of compressed air that damages enemies at range
---

# Details

- Damage: 30
- Range: Medium
- Learned from: Zephyr (companion)
EOF

cat > entities/abilities/ABL-002.md << 'EOF'
---
id: ABL-002
name: Gale Force
category: combat
cooldown: 15
description: Creates a powerful burst of wind that knocks back all nearby enemies
---

# Details

- Damage: 20 (AoE)
- Effect: Knockback
- Learned from: Elder Nimbus
EOF

cat > entities/abilities/ABL-003.md << 'EOF'
---
id: ABL-003
name: Tailwind
category: utility
cooldown: 30
description: Increases movement speed for the entire party
---

# Details

- Duration: 20 seconds
- Effect: +50% movement speed
- Learned from: Zephyr (companion)
EOF

cat > entities/abilities/ABL-004.md << 'EOF'
---
id: ABL-004
name: Sky Walk
category: utility
cooldown: 10
description: Allows brief walking on air currents
---

# Details

- Duration: 3 seconds
- Unlocked by: Completing Trial of Breath
EOF

cat > entities/abilities/ABL-005.md << 'EOF'
---
id: ABL-005
name: Storm's Fury
category: ultimate
cooldown: 120
description: Calls down a devastating lightning storm
---

# Details

- Damage: 200 (large AoE)
- Requires: Heart of Storms
- Learned from: Elder Nimbus (after main quest)
EOF

git add entities/abilities/
git commit -m "Add abilities" > /dev/null

#############################################################################
# PHASE 7: Lore
#############################################################################
echo "Creating lore entries..."

mkdir -p entities/lore

cat > entities/lore/LORE-001.md << 'EOF'
---
id: LORE-001
title: The Sundering
era: Ancient History
description: The cataclysm that shattered the world
---

Long ago, the world was whole - a vast continent surrounded by endless seas. Then came the Sundering, a cataclysm of unknown origin that shattered the land itself.

The pieces rose into the sky, held aloft by mysterious forces. The seas fell away into an endless abyss. Those who survived found themselves on floating islands, forever separated from the ground they once knew.
EOF

cat > entities/lore/LORE-002.md << 'EOF'
---
id: LORE-002
title: The Five Hearts
era: Ancient History
description: Artifacts of immense power created by the ancients
---

The ancient skyward civilization created five artifacts of immense power, each containing the essence of a primal element:

- Heart of Storms (Wind)
- Heart of Embers (Fire)
- Heart of Tides (Water)
- Heart of Stone (Earth)
- Heart of Darkness (Void)

These hearts were hidden across the world to prevent their power from being misused.
EOF

cat > entities/lore/LORE-003.md << 'EOF'
---
id: LORE-003
title: The Cloudkeeper Order
era: Modern Era
description: Guardians of wind magic and ancient knowledge
---

Founded centuries after the Sundering, the Cloudkeeper Order dedicated themselves to preserving the knowledge and magic of the ancient civilization.

Their monastery, the Temple of Winds, was built at the highest point of the floating islands, closest to the sun and the pure winds.
EOF

cat > entities/lore/LORE-004.md << 'EOF'
---
id: LORE-004
title: Rise of the Iron Fleet
era: Recent History
description: The emergence of Admiral Korrath's military force
---

Twenty years ago, a naval officer named Korrath grew disillusioned with the chaotic state of the floating islands. He believed that only through strength and unity could the people survive.

He gathered like-minded individuals and began building the Iron Fleet - a massive armada of warships designed to bring order through force.
EOF

git add entities/lore/
git commit -m "Add lore entries" > /dev/null

#############################################################################
# PHASE 8: Achievements
#############################################################################
echo "Creating achievements..."

mkdir -p entities/achievements

cat > entities/achievements/ACH-001.md << 'EOF'
---
id: ACH-001
title: First Steps
category: story
points: 10
hidden: false
description: Complete the tutorial quest
---

Awarded automatically upon completing "A Storm on the Horizon".
EOF

cat > entities/achievements/ACH-002.md << 'EOF'
---
id: ACH-002
title: Sky Explorer
category: exploration
points: 25
hidden: false
description: Visit all major locations
---

Requires visiting:
- Windhollow
- Shattered Isles
- Temple of Winds
- Ironclad Fortress
- The Abyss
EOF

cat > entities/achievements/ACH-003.md << 'EOF'
---
id: ACH-003
title: Dragon Slayer
category: combat
points: 50
hidden: true
description: Defeat the Shadow Wyrm
---

Optional achievement for defeating the optional boss.
EOF

cat > entities/achievements/ACH-004.md << 'EOF'
---
id: ACH-004
title: Collector's Edition
category: collection
points: 30
hidden: false
description: Find all lore entries
---

Requires finding all 4 lore entries scattered throughout the world.
EOF

cat > entities/achievements/ACH-005.md << 'EOF'
---
id: ACH-005
title: True Ending
category: story
points: 100
hidden: true
description: Discover the secret ending
---

Requires making specific choices throughout the game and collecting all Five Hearts.
EOF

git add entities/achievements/
git commit -m "Add achievements" > /dev/null

#############################################################################
# PHASE 9: Relations
#############################################################################
echo "Creating relations..."

# Character locations
cat > "relations/CHAR-001--found-at--LOC-001.md" << 'EOF'
---
from: CHAR-001
type: found-at
to: LOC-001
---
EOF

cat > "relations/CHAR-002--found-at--LOC-001.md" << 'EOF'
---
from: CHAR-002
type: found-at
to: LOC-001
---
EOF

cat > "relations/CHAR-003--found-at--LOC-004.md" << 'EOF'
---
from: CHAR-003
type: found-at
to: LOC-004
---
EOF

cat > "relations/CHAR-004--found-at--LOC-003.md" << 'EOF'
---
from: CHAR-004
type: found-at
to: LOC-003
---
EOF

cat > "relations/CHAR-005--found-at--LOC-005.md" << 'EOF'
---
from: CHAR-005
type: found-at
to: LOC-005
---
EOF

cat > "relations/CHAR-006--found-at--LOC-003.md" << 'EOF'
---
from: CHAR-006
type: found-at
to: LOC-003
---
EOF

cat > "relations/CHAR-007--found-at--LOC-002.md" << 'EOF'
---
from: CHAR-007
type: found-at
to: LOC-002
---
EOF

cat > "relations/CHAR-008--found-at--LOC-001.md" << 'EOF'
---
from: CHAR-008
type: found-at
to: LOC-001
---
EOF

# Quest givers
cat > "relations/CHAR-001--gives-quest--QUEST-001.md" << 'EOF'
---
from: CHAR-001
type: gives-quest
to: QUEST-001
---
EOF

cat > "relations/CHAR-001--gives-quest--QUEST-002.md" << 'EOF'
---
from: CHAR-001
type: gives-quest
to: QUEST-002
---
EOF

cat > "relations/CHAR-006--gives-quest--QUEST-003.md" << 'EOF'
---
from: CHAR-006
type: gives-quest
to: QUEST-003
---
EOF

cat > "relations/CHAR-001--gives-quest--QUEST-004.md" << 'EOF'
---
from: CHAR-001
type: gives-quest
to: QUEST-004
---
EOF

cat > "relations/CHAR-006--gives-quest--QUEST-005.md" << 'EOF'
---
from: CHAR-006
type: gives-quest
to: QUEST-005
---
EOF

cat > "relations/CHAR-002--gives-quest--QUEST-006.md" << 'EOF'
---
from: CHAR-002
type: gives-quest
to: QUEST-006
---
EOF

# Quest locations
cat > "relations/QUEST-001--takes-place-in--LOC-001.md" << 'EOF'
---
from: QUEST-001
type: takes-place-in
to: LOC-001
---
EOF

cat > "relations/QUEST-002--takes-place-in--LOC-002.md" << 'EOF'
---
from: QUEST-002
type: takes-place-in
to: LOC-002
---
EOF

cat > "relations/QUEST-003--takes-place-in--LOC-003.md" << 'EOF'
---
from: QUEST-003
type: takes-place-in
to: LOC-003
---
EOF

cat > "relations/QUEST-004--takes-place-in--LOC-004.md" << 'EOF'
---
from: QUEST-004
type: takes-place-in
to: LOC-004
---
EOF

cat > "relations/QUEST-005--takes-place-in--LOC-005.md" << 'EOF'
---
from: QUEST-005
type: takes-place-in
to: LOC-005
---
EOF

cat > "relations/QUEST-006--takes-place-in--LOC-002.md" << 'EOF'
---
from: QUEST-006
type: takes-place-in
to: LOC-002
---
EOF

# Quest rewards
cat > "relations/QUEST-001--rewards--ITEM-001.md" << 'EOF'
---
from: QUEST-001
type: rewards
to: ITEM-001
---
EOF

cat > "relations/QUEST-002--rewards--ITEM-002.md" << 'EOF'
---
from: QUEST-002
type: rewards
to: ITEM-002
---
EOF

cat > "relations/QUEST-003--rewards--ITEM-003.md" << 'EOF'
---
from: QUEST-003
type: rewards
to: ITEM-003
---
EOF

cat > "relations/QUEST-006--rewards--ITEM-007.md" << 'EOF'
---
from: QUEST-006
type: rewards
to: ITEM-007
---
EOF

# Character teaches
cat > "relations/CHAR-004--teaches--ABL-001.md" << 'EOF'
---
from: CHAR-004
type: teaches
to: ABL-001
---
EOF

cat > "relations/CHAR-004--teaches--ABL-003.md" << 'EOF'
---
from: CHAR-004
type: teaches
to: ABL-003
---
EOF

cat > "relations/CHAR-006--teaches--ABL-002.md" << 'EOF'
---
from: CHAR-006
type: teaches
to: ABL-002
---
EOF

cat > "relations/CHAR-006--teaches--ABL-005.md" << 'EOF'
---
from: CHAR-006
type: teaches
to: ABL-005
---
EOF

# Unlocks
cat > "relations/QUEST-003--unlocks--LOC-004.md" << 'EOF'
---
from: QUEST-003
type: unlocks
to: LOC-004
---
EOF

cat > "relations/QUEST-003--unlocks--ABL-004.md" << 'EOF'
---
from: QUEST-003
type: unlocks
to: ABL-004
---
EOF

# Requirements
cat > "relations/QUEST-005--requires--ITEM-005.md" << 'EOF'
---
from: QUEST-005
type: requires
to: ITEM-005
---
EOF

cat > "relations/ABL-005--requires--ITEM-006.md" << 'EOF'
---
from: ABL-005
type: requires
to: ITEM-006
---
EOF

# Triggers (achievements)
cat > "relations/QUEST-001--triggers--ACH-001.md" << 'EOF'
---
from: QUEST-001
type: triggers
to: ACH-001
---
EOF

cat > "relations/QUEST-005--triggers--ACH-002.md" << 'EOF'
---
from: QUEST-005
type: triggers
to: ACH-002
---
EOF

# Drops
cat > "relations/CHAR-005--drops--ITEM-005.md" << 'EOF'
---
from: CHAR-005
type: drops
to: ITEM-005
---
EOF

cat > "relations/CHAR-007--drops--ITEM-008.md" << 'EOF'
---
from: CHAR-007
type: drops
to: ITEM-008
---
EOF

# Sells
cat > "relations/CHAR-002--sells--ITEM-004.md" << 'EOF'
---
from: CHAR-002
type: sells
to: ITEM-004
---
EOF

# Guards
cat > "relations/CHAR-005--guards--LOC-005.md" << 'EOF'
---
from: CHAR-005
type: guards
to: LOC-005
---
EOF

cat > "relations/CHAR-003--guards--LOC-004.md" << 'EOF'
---
from: CHAR-003
type: guards
to: LOC-004
---
EOF

# Lore mentions
cat > "relations/LORE-001--mentions--LOC-005.md" << 'EOF'
---
from: LORE-001
type: mentions
to: LOC-005
---
EOF

cat > "relations/LORE-002--mentions--ITEM-006.md" << 'EOF'
---
from: LORE-002
type: mentions
to: ITEM-006
---
EOF

cat > "relations/LORE-003--mentions--CHAR-006.md" << 'EOF'
---
from: LORE-003
type: mentions
to: CHAR-006
---
EOF

cat > "relations/LORE-003--mentions--LOC-003.md" << 'EOF'
---
from: LORE-003
type: mentions
to: LOC-003
---
EOF

cat > "relations/LORE-004--mentions--CHAR-003.md" << 'EOF'
---
from: LORE-004
type: mentions
to: CHAR-003
---
EOF

git add relations/
git commit -m "Add relations" > /dev/null

#############################################################################
# PHASE 10: Push to origin
#############################################################################
echo "Pushing to origin..."
git push -u origin main > /dev/null 2>&1

#############################################################################
# PHASE 11: Leave uncommitted changes for demo
#############################################################################
echo "Creating uncommitted changes for demo..."

# Modify a quest status (simulates work in progress)
sed -i.bak 's/status: draft/status: testing/' entities/quests/QUEST-003.md
rm entities/quests/QUEST-003.md.bak

# Add a new character (simulates new content)
cat > entities/characters/CHAR-009.md << 'EOF'
---
id: CHAR-009
name: Bolt the Messenger
role: npc
level: 5
essential: false
description: A speedy courier who delivers messages across the islands
---

# Background

Bolt is known throughout the floating islands as the fastest courier. If you need a message delivered quickly, he's your bird... er, person.

# Services

- Fast travel unlocks
- Delivers quest items between locations
EOF

#############################################################################
# Done!
#############################################################################
echo ""
echo "=== Skyward Chronicles Demo Ready! ==="
echo ""
echo "  Origin: $ORIGIN_DIR"
echo "  Project: $PROJECT_DIR"
echo ""
echo "Git status:"
cd "$PROJECT_DIR"
git status --short
echo ""
echo "Run the server:"
echo "  go run ./cmd/rela-server -project $PROJECT_DIR -port 9080"
echo ""
echo "Then open: http://localhost:9080"
echo ""
