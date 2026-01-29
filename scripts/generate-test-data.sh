#!/bin/bash
set -e

# generate-test-data.sh - Generate a rela project populated with test data
#
# Creates a rela project directory with a metamodel and the specified number
# of entities and relations, useful for performance testing and demos.
#
# Usage:
#   ./scripts/generate-test-data.sh [options] <output-dir>
#
# Options:
#   -e, --entities NUM     Number of entities to generate (default: 100)
#   -r, --relations NUM    Number of relations to generate (default: 150)
#   -s, --seed NUM         Random seed for reproducibility (default: random)
#   -h, --help             Show this help message
#
# Examples:
#   ./scripts/generate-test-data.sh /tmp/rela-test           # 100 entities, 150 relations
#   ./scripts/generate-test-data.sh -e 1000 -r 2000 /tmp/big # Large dataset
#   ./scripts/generate-test-data.sh -e 10 -r 15 /tmp/small   # Small dataset
#   ./scripts/generate-test-data.sh -s 42 /tmp/reproducible   # Reproducible output

# --- Defaults ---
NUM_ENTITIES=100
NUM_RELATIONS=150
SEED=""
OUTPUT_DIR=""

# --- Word banks for generating realistic content ---

REQ_TITLES=(
  "System shall support concurrent user sessions"
  "Data must be encrypted at rest and in transit"
  "API response time shall not exceed 200ms"
  "System shall provide audit logging"
  "User authentication via OAuth 2.0"
  "Support horizontal scaling to 10k users"
  "Automated backup every 24 hours"
  "GDPR compliance for personal data handling"
  "Role-based access control for all resources"
  "Real-time notification delivery"
  "Search functionality with full-text indexing"
  "Multi-tenant data isolation"
  "Support for internationalization and localization"
  "System availability of 99.9% uptime"
  "Mobile-responsive user interface"
  "Integration with third-party payment gateway"
  "Batch processing of large datasets"
  "WebSocket support for live updates"
  "Rate limiting on public API endpoints"
  "Configurable retention policies for data"
  "Support for file uploads up to 100MB"
  "Two-factor authentication support"
  "Automated vulnerability scanning"
  "Data export in CSV and JSON formats"
  "Session timeout after 30 minutes of inactivity"
  "Password complexity requirements"
  "Graceful degradation under load"
  "Cross-browser compatibility"
  "Accessibility compliance with WCAG 2.1"
  "Database migration tooling support"
)

DEC_TITLES=(
  "Use PostgreSQL as primary database"
  "Adopt microservices architecture"
  "Implement event-driven communication"
  "Use Kubernetes for container orchestration"
  "Select React for frontend framework"
  "Implement CQRS pattern for read/write separation"
  "Use Redis for caching layer"
  "Adopt OpenTelemetry for observability"
  "Use gRPC for internal service communication"
  "Implement circuit breaker pattern"
  "Use Terraform for infrastructure as code"
  "Adopt trunk-based development workflow"
  "Use JWT tokens for API authentication"
  "Implement saga pattern for distributed transactions"
  "Select Kafka for event streaming"
  "Use GraphQL for public API"
  "Implement blue-green deployment strategy"
  "Adopt domain-driven design principles"
  "Use S3 for object storage"
  "Implement feature flags for progressive rollout"
)

SOL_TITLES=(
  "Authentication service implementation"
  "API gateway with rate limiting"
  "Event bus using message queues"
  "Centralized logging pipeline"
  "Database connection pooling"
  "CDN integration for static assets"
  "Automated CI/CD pipeline"
  "Health check and monitoring endpoints"
  "Data migration framework"
  "Search indexing service"
  "Notification delivery system"
  "File storage abstraction layer"
  "Configuration management service"
  "Background job processing system"
  "API versioning strategy implementation"
  "Cache invalidation mechanism"
  "Error tracking and alerting system"
  "Database sharding implementation"
  "Load balancer configuration"
  "Secrets management solution"
)

COMP_TITLES=(
  "User service"
  "Auth gateway"
  "Payment processor"
  "Notification engine"
  "Search service"
  "File storage service"
  "Analytics pipeline"
  "Admin dashboard"
  "API gateway"
  "Message broker"
  "Cache layer"
  "Database cluster"
  "CDN edge nodes"
  "Monitoring stack"
  "Logging aggregator"
  "Config server"
  "Job scheduler"
  "Email service"
  "Webhook dispatcher"
  "Rate limiter"
)

RATIONALES=(
  "Chosen for proven scalability and community support"
  "Best fit for our team's existing expertise"
  "Provides the best cost-performance ratio"
  "Industry standard with strong ecosystem"
  "Simplifies maintenance and reduces operational overhead"
  "Aligns with our long-term technology strategy"
  "Recommended by architecture review board"
  "Minimizes vendor lock-in risk"
  "Enables faster time to market"
  "Strongest security posture among alternatives"
)

DESCRIPTIONS=(
  "This addresses a critical business requirement identified during stakeholder review."
  "Ensures compliance with regulatory standards and industry best practices."
  "Key enabler for the next phase of platform development."
  "Reduces technical debt and improves maintainability."
  "Critical for meeting performance SLAs under peak load conditions."
  "Supports the organization's digital transformation initiative."
  "Improves developer experience and reduces onboarding time."
  "Enables better observability and faster incident response."
  "Required for upcoming product launch timeline."
  "Addresses feedback from security audit findings."
)

STATUSES=(draft proposed accepted deprecated rejected retired)
PRIORITIES=(critical high medium low)

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
  case $1 in
    -e|--entities)
      NUM_ENTITIES="$2"
      shift 2
      ;;
    -r|--relations)
      NUM_RELATIONS="$2"
      shift 2
      ;;
    -s|--seed)
      SEED="$2"
      shift 2
      ;;
    -h|--help)
      head -18 "$0" | tail -16 | sed 's/^# \?//'
      exit 0
      ;;
    -*)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
    *)
      OUTPUT_DIR="$1"
      shift
      ;;
  esac
done

if [ -z "$OUTPUT_DIR" ]; then
  echo "Error: output directory is required" >&2
  echo "Usage: $0 [options] <output-dir>" >&2
  exit 1
fi

# --- Seed random number generator ---
if [ -n "$SEED" ]; then
  RANDOM=$SEED
fi

# --- Helpers ---
# All random helpers set a global _RESULT variable instead of echoing,
# to avoid subshells which break RANDOM seed reproducibility.

_RESULT=""

pick_random() {
  local arr=("$@")
  _RESULT="${arr[$((RANDOM % ${#arr[@]}))]}"
}

pick_status() {
  # Weighted: more draft/proposed/accepted than deprecated/rejected/retired
  local weighted=(draft draft draft proposed proposed proposed accepted accepted accepted deprecated rejected retired)
  pick_random "${weighted[@]}"
}

pick_priority() {
  local weighted=(medium medium medium high high low low critical)
  pick_random "${weighted[@]}"
}

# Sets _RESULT to the formatted ID
format_id() {
  local prefix=$1
  local num=$2
  local width=$3
  printf -v _RESULT "%s%0${width}d" "$prefix" "$num"
}

# --- Setup output directory ---

if [ -d "$OUTPUT_DIR" ]; then
  echo "Warning: $OUTPUT_DIR already exists, files may be overwritten"
fi

mkdir -p "$OUTPUT_DIR"/{entities/{requirement,decision,solution,component},relations,.rela}

# --- Write metamodel.yaml ---

cat > "$OUTPUT_DIR/metamodel.yaml" << 'METAMODEL'
# Architecture Metamodel - Generated for testing
# This file defines the entity types, relations, and validation rules.

version: "1.0"
namespace: "https://example.org/ontology/architecture#"

types:
  status:
    values: [draft, proposed, accepted, deprecated, rejected, retired]
    default: draft

  priority:
    values: [critical, high, medium, low]

entities:
  requirement:
    label: Requirement
    aliases: [req]
    id_prefix: "REQ-"
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      status:
        type: status
        required: true
      priority:
        type: priority

  decision:
    label: Decision
    aliases: [dec, adr]
    id_prefixes: ["DEC-", "ADR-"]
    properties:
      title:
        type: string
        required: true
      rationale:
        type: string
      status:
        type: status
        required: true

  solution:
    label: Solution
    aliases: [sol]
    id_prefix: "SOL-"
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      status:
        type: status

  component:
    label: Component
    aliases: [comp]
    id_prefixes: ["COMP-", "AC-", "TC-"]
    properties:
      title:
        type: string
        required: true

relations:
  addresses:
    label: addresses
    description: A decision addresses a requirement
    from: [decision]
    to: [requirement]
    inverse: addressedBy

  implements:
    label: implements
    description: A solution implements a decision
    from: [solution]
    to: [decision]
    inverse: implementedBy

  realizes:
    label: realizes
    description: A component realizes a solution
    from: [component]
    to: [solution]
    inverse: realizedBy

  dependsOn:
    label: depends on
    from: [component, solution, decision]
    to: [component, solution, decision]
    inverse: dependencyOf
METAMODEL

# --- Distribute entities across types ---
# Roughly: 30% requirements, 25% decisions, 25% solutions, 20% components

NUM_REQ=$((NUM_ENTITIES * 30 / 100))
NUM_DEC=$((NUM_ENTITIES * 25 / 100))
NUM_SOL=$((NUM_ENTITIES * 25 / 100))
NUM_COMP=$((NUM_ENTITIES - NUM_REQ - NUM_DEC - NUM_SOL))

# Ensure at least 1 of each type
[ "$NUM_REQ" -lt 1 ] && NUM_REQ=1
[ "$NUM_DEC" -lt 1 ] && NUM_DEC=1
[ "$NUM_SOL" -lt 1 ] && NUM_SOL=1
[ "$NUM_COMP" -lt 1 ] && NUM_COMP=1

if [ "$NUM_ENTITIES" -le 999 ]; then
  WIDTH=3
elif [ "$NUM_ENTITIES" -le 9999 ]; then
  WIDTH=4
elif [ "$NUM_ENTITIES" -le 99999 ]; then
  WIDTH=5
else
  WIDTH=6
fi

echo "Generating test data in $OUTPUT_DIR"
echo "  Entities: $NUM_ENTITIES (REQ:$NUM_REQ DEC:$NUM_DEC SOL:$NUM_SOL COMP:$NUM_COMP)"
echo "  Relations: $NUM_RELATIONS"
echo ""

# --- Collect all entity IDs for relation generation ---
ALL_REQ_IDS=()
ALL_DEC_IDS=()
ALL_SOL_IDS=()
ALL_COMP_IDS=()

# --- Generate requirements ---
echo -n "  Generating requirements..."
for i in $(seq 1 "$NUM_REQ"); do
  format_id "REQ-" "$i" "$WIDTH"; ID="$_RESULT"
  ALL_REQ_IDS+=("$ID")
  pick_random "${REQ_TITLES[@]}"; TITLE="$_RESULT"
  pick_status; STATUS="$_RESULT"
  pick_priority; PRIORITY="$_RESULT"
  pick_random "${DESCRIPTIONS[@]}"; DESC="$_RESULT"

  cat > "$OUTPUT_DIR/entities/requirement/$ID.md" << EOF
---
id: $ID
type: requirement
title: "$TITLE"
status: $STATUS
priority: $PRIORITY
description: "$DESC"
---

# $TITLE

$DESC

## Acceptance Criteria

- Criterion $i.1: Verified through automated testing
- Criterion $i.2: Validated by stakeholder review
- Criterion $i.3: Confirmed via load testing
EOF
done
echo " done ($NUM_REQ)"

# --- Generate decisions ---
echo -n "  Generating decisions..."
for i in $(seq 1 "$NUM_DEC"); do
  format_id "DEC-" "$i" "$WIDTH"; ID="$_RESULT"
  ALL_DEC_IDS+=("$ID")
  pick_random "${DEC_TITLES[@]}"; TITLE="$_RESULT"
  pick_status; STATUS="$_RESULT"
  pick_random "${RATIONALES[@]}"; RATIONALE="$_RESULT"

  cat > "$OUTPUT_DIR/entities/decision/$ID.md" << EOF
---
id: $ID
type: decision
title: "$TITLE"
status: $STATUS
rationale: "$RATIONALE"
---

# $TITLE

## Context

A decision was needed regarding the system architecture.

## Decision

$TITLE.

## Rationale

$RATIONALE.

## Consequences

- Positive: Improved maintainability and scalability
- Negative: Additional complexity in initial setup
EOF
done
echo " done ($NUM_DEC)"

# --- Generate solutions ---
echo -n "  Generating solutions..."
for i in $(seq 1 "$NUM_SOL"); do
  format_id "SOL-" "$i" "$WIDTH"; ID="$_RESULT"
  ALL_SOL_IDS+=("$ID")
  pick_random "${SOL_TITLES[@]}"; TITLE="$_RESULT"
  pick_status; STATUS="$_RESULT"
  pick_random "${DESCRIPTIONS[@]}"; DESC="$_RESULT"

  cat > "$OUTPUT_DIR/entities/solution/$ID.md" << EOF
---
id: $ID
type: solution
title: "$TITLE"
status: $STATUS
description: "$DESC"
---

# $TITLE

$DESC

## Implementation Notes

- Phase 1: Core functionality
- Phase 2: Integration testing
- Phase 3: Production rollout
EOF
done
echo " done ($NUM_SOL)"

# --- Generate components ---
echo -n "  Generating components..."
for i in $(seq 1 "$NUM_COMP"); do
  format_id "COMP-" "$i" "$WIDTH"; ID="$_RESULT"
  ALL_COMP_IDS+=("$ID")
  pick_random "${COMP_TITLES[@]}"; TITLE="$_RESULT"

  cat > "$OUTPUT_DIR/entities/component/$ID.md" << EOF
---
id: $ID
type: component
title: "$TITLE"
---

# $TITLE

Runtime component responsible for $TITLE functionality.
EOF
done
echo " done ($NUM_COMP)"

# --- Generate relations ---
# Relation types and their valid from->to mappings:
#   addresses:  decision   -> requirement
#   implements: solution   -> decision
#   realizes:   component  -> solution
#   dependsOn:  (comp|sol|dec) -> (comp|sol|dec)

echo -n "  Generating relations..."

generated=0
attempts=0
max_attempts=$((NUM_RELATIONS * 10))

# Distribute relations: ~30% addresses, ~25% implements, ~25% realizes, ~20% dependsOn
target_addresses=$((NUM_RELATIONS * 30 / 100))
target_implements=$((NUM_RELATIONS * 25 / 100))
target_realizes=$((NUM_RELATIONS * 25 / 100))
target_depends=$((NUM_RELATIONS - target_addresses - target_implements - target_realizes))

count_addresses=0
count_implements=0
count_realizes=0
count_depends=0

while [ "$generated" -lt "$NUM_RELATIONS" ] && [ "$attempts" -lt "$max_attempts" ]; do
  attempts=$((attempts + 1))

  # Pick relation type based on remaining targets
  if [ "$count_addresses" -lt "$target_addresses" ]; then
    REL_TYPE="addresses"
  elif [ "$count_implements" -lt "$target_implements" ]; then
    REL_TYPE="implements"
  elif [ "$count_realizes" -lt "$target_realizes" ]; then
    REL_TYPE="realizes"
  elif [ "$count_depends" -lt "$target_depends" ]; then
    REL_TYPE="dependsOn"
  else
    # All targets met, pick randomly for remaining
    case $((RANDOM % 4)) in
      0) REL_TYPE="addresses" ;;
      1) REL_TYPE="implements" ;;
      2) REL_TYPE="realizes" ;;
      3) REL_TYPE="dependsOn" ;;
    esac
  fi

  # Pick valid from/to based on relation type
  case "$REL_TYPE" in
    addresses)
      [ "${#ALL_DEC_IDS[@]}" -eq 0 ] || [ "${#ALL_REQ_IDS[@]}" -eq 0 ] && continue
      pick_random "${ALL_DEC_IDS[@]}"; FROM_ID="$_RESULT"
      pick_random "${ALL_REQ_IDS[@]}"; TO_ID="$_RESULT"
      ;;
    implements)
      [ "${#ALL_SOL_IDS[@]}" -eq 0 ] || [ "${#ALL_DEC_IDS[@]}" -eq 0 ] && continue
      pick_random "${ALL_SOL_IDS[@]}"; FROM_ID="$_RESULT"
      pick_random "${ALL_DEC_IDS[@]}"; TO_ID="$_RESULT"
      ;;
    realizes)
      [ "${#ALL_COMP_IDS[@]}" -eq 0 ] || [ "${#ALL_SOL_IDS[@]}" -eq 0 ] && continue
      pick_random "${ALL_COMP_IDS[@]}"; FROM_ID="$_RESULT"
      pick_random "${ALL_SOL_IDS[@]}"; TO_ID="$_RESULT"
      ;;
    dependsOn)
      # dependsOn can link component, solution, or decision to each other
      ALL_DEPS=("${ALL_COMP_IDS[@]}" "${ALL_SOL_IDS[@]}" "${ALL_DEC_IDS[@]}")
      [ "${#ALL_DEPS[@]}" -lt 2 ] && continue
      pick_random "${ALL_DEPS[@]}"; FROM_ID="$_RESULT"
      pick_random "${ALL_DEPS[@]}"; TO_ID="$_RESULT"
      # No self-references
      [ "$FROM_ID" = "$TO_ID" ] && continue
      ;;
  esac

  # Check for duplicate by testing if file already exists
  REL_KEY="${FROM_ID}--${REL_TYPE}--${TO_ID}"
  if [ -f "$OUTPUT_DIR/relations/$REL_KEY.md" ]; then
    continue
  fi

  # Write relation file
  cat > "$OUTPUT_DIR/relations/$REL_KEY.md" << EOF
---
from: $FROM_ID
relation: $REL_TYPE
to: $TO_ID
---
EOF

  generated=$((generated + 1))

  case "$REL_TYPE" in
    addresses) count_addresses=$((count_addresses + 1)) ;;
    implements) count_implements=$((count_implements + 1)) ;;
    realizes) count_realizes=$((count_realizes + 1)) ;;
    dependsOn) count_depends=$((count_depends + 1)) ;;
  esac
done

echo " done ($generated)"

if [ "$generated" -lt "$NUM_RELATIONS" ]; then
  echo "  Note: only $generated relations generated (not enough unique combinations for $NUM_RELATIONS)"
fi

# --- Summary ---
echo ""
echo "Test data generated successfully:"
echo "  Directory:    $OUTPUT_DIR"
echo "  Metamodel:    $OUTPUT_DIR/metamodel.yaml"
echo "  Requirements: $NUM_REQ"
echo "  Decisions:    $NUM_DEC"
echo "  Solutions:    $NUM_SOL"
echo "  Components:   $NUM_COMP"
echo "  Relations:    $generated"
echo ""
echo "To use: cd $OUTPUT_DIR && rela list"
