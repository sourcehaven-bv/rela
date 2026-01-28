# View Engine Issues Found During Integration Testing

## Context

These issues were discovered while integrating the `rela view` feature with a
real-world document publishing pipeline (GF Adressering project). The test
involved replacing a 665-line Python script with the declarative view system.

## Issue 1: `collect_as` Does Not Filter by Entity Type

### Description

When using `collect_as` with multiple collection names, all traversed entities
are added to **all** named collections, regardless of their entity type.

### Example

```yaml
traverse:
  - from: bouwbloks
    follow_incoming: partOfBouwblok
    collect_as: [functions, usecases, scenarios]
```

### Expected Behavior

- `functions` collection should only contain entities of type `function`
- `usecases` collection should only contain entities of type `usecase`
- `scenarios` collection should only contain entities of type `scenario`

### Actual Behavior

All three collections contain a mix of all entity types:

```
functions: {'function': 9, 'usecase': 10, 'scenario': 1, 'component': 1}
usecases: {'function': 9, 'usecase': 28, 'scenario': 1, 'component': 1}
scenarios: {'function': 9, 'usecase': 10, 'scenario': 1, 'component': 1}
```

### Impact

- Significantly inflated collection sizes (e.g., 1874 glossary terms instead
  of 65)
- Wrong entities rendered in document sections
- Requires post-processing workaround to filter by type

### Workaround

Filter collections by entity type in post-processing:

```python
def filter_by_type(entities: list, entity_type: str) -> list:
    return [e for e in entities if e.get('type') == entity_type]

context['rela_functions'] = filter_by_type(collections.get('functions', []), 'function')
```

### Suggested Fix

In `engine.go`, when adding entities to collections, check if the collection
name matches the entity type:

```go
// In applyTraverseRule, when adding to collections:
for _, collectAs := range collectAsNames {
    for _, entity := range foundEntities {
        // Only add if collection name matches entity type (singular/plural)
        if matchesCollectionType(collectAs, entity.Type) {
            result.Collections[collectAs] = append(result.Collections[collectAs], entity)
        }
    }
}
```

---

## Issue 2: Filter `id_prefix` Does Not Expand Collections

### Description

The `id_prefix` filter in `match_any` only **filters** entities already in a
collection. It does not **add** entities from the graph that match the prefix
but weren't reached via traversal.

### Example

```yaml
filters:
  requirements:
    match_any:
      - via_traversal: true
      - id_prefix: ["LRZA-", "GF-"]
```

### Expected Behavior

All requirements with IDs starting with `LRZA-` or `GF-` should be included in
the `requirements` collection, even if not reached via traverse rules.

### Actual Behavior

Only 1 requirement (reached via traversal) is included, instead of the
expected 19.

### Impact

Critical for use cases where entities should be included based on naming
convention rather than graph connectivity.

### Workaround

Manually fetch and add entities matching the prefix:

```python
all_requirements = get_all_requirements(base_path)
for req in all_requirements:
    req_id = req.get('id', '')
    for prefix in prefixes:
        if req_id.startswith(prefix):
            supplemented_requirements.append(req)
            break
```

### Suggested Fix

Add an `expand` mode to filters that queries the graph for matching entities:

```yaml
filters:
  requirements:
    expand:  # New mode: add entities from graph
      id_prefix: ["LRZA-", "GF-"]
```

Or support a separate `include` section:

```yaml
include:
  requirements:
    id_prefix: ["LRZA-", "GF-"]
```

---

## Issue 3: Traversal Order Affects Reachability

### Description

Entities reachable via indirect paths may not be found if the traverse rules are
not ordered correctly, or if intermediate entities aren't collected first.

### Example

The following entities were not found by the view but were found by the original
script:

| Entity         | Expected Path                                                 |
| -------------- | ------------------------------------------------------------- |
| `GF-COMP-001`  | persona → usesFunction → function → realizes → component      |
| `GF-COMP-002`  | persona → usesFunction → component (direct)                   |
| `LRZA-ISS-002` | issue → issueAffects → component (component not in scope yet) |
| `LRZA-UC-009`  | indirect path via multiple relations                          |

### Root Cause

The view engine processes traverse rules sequentially. If a rule references a
collection that isn't fully populated yet, some entities may be missed.

### Impact

~4 entities (out of ~100) were missing, representing 4% of content.

### Suggested Fixes

1. **Multi-pass traversal**: Run traverse rules multiple times until no new
   entities are found
2. **Dependency ordering**: Analyze traverse rules and reorder based on
   dependencies
3. **Explicit collection references**: Allow
   `from: [components, _pending_components]` syntax

---

## Test Results Summary

| Metric            | Original Script | View-based | Delta |
| ----------------- | --------------- | ---------- | ----- |
| Lines of code     | 665             | 180        | -73%  |
| Subprocess calls  | ~100            | 2          | -98%  |
| Entities matched  | 100%            | 95.8%      | -4.2% |
| Document sections | 100%            | 95%        | -5%   |

The view feature is **production-viable** with the workarounds, but these fixes
would eliminate the need for post-processing.

---

## Reproduction Steps

1. Use the `views.yaml` from the GF Adressering project
2. Run: `rela view document_publish DOC-001 -o yaml`
3. Compare entity counts with the original Python script output

## Files

- Test project: `psa-adressering` (GF Adressering)
- View definition: `views.yaml`
- Workaround script: `publish/generate_context_for_doc_v2.py`
