<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Export Guide

The `rela export` command enables powerful reporting and analysis by exporting your
entities to standard formats (JSON, CSV, YAML) that work with common data tools.

## Quick Start

```bash
# Export all requirements as JSON
rela export requirement --format json

# Export with relations included
rela export control --with-relations

# Export everything
rela export --all --format json
```

## Output Formats

### JSON (default)

Best for programmatic processing with tools like `jq`.

```bash
rela export control --format json
```

Output:

```json
[
  {
    "id": "CTRL-001",
    "type": "control",
    "properties": {
      "title": "Access Control Policy",
      "status": "implemented",
      "iso27001": "A.5.15"
    }
  }
]
```

### CSV

Best for spreadsheets and tools like Miller (`mlr`), `xsv`, or pandas.

```bash
rela export control --format csv
```

Output:

```csv
id,type,title,status,iso27001
CTRL-001,control,Access Control Policy,implemented,A.5.15
CTRL-002,control,Password Policy,draft,A.9.4.3
```

### YAML

Best for human readability and configuration files.

```bash
rela export control --format yaml
```

Output:

```yaml
- id: CTRL-001
  type: control
  properties:
    title: Access Control Policy
    status: implemented
    iso27001: A.5.15
```

## Including Relations

Add `--with-relations` to include relationship data:

```bash
rela export control --with-relations --format json
```

This adds a `relations` field to each entity:

```json
{
  "id": "CTRL-001",
  "type": "control",
  "properties": { ... },
  "relations": {
    "outgoing": {
      "mitigates": [
        {"id": "RISK-001", "title": "Unauthorized Access"}
      ],
      "evidencedBy": [
        {"id": "EV-001", "title": "Access Control Audit Report"}
      ]
    },
    "incoming": {
      "implements": [
        {"id": "PROC-001", "title": "Access Control Procedure"}
      ]
    }
  }
}
```

For CSV, relations are compressed into semicolon-separated values:

```csv
id,type,title,status,relations_outgoing,relations_incoming
CTRL-001,control,Access Control Policy,implemented,evidencedBy:EV-001;mitigates:RISK-001,implements:PROC-001
```

## Full Graph Export

Export everything with `--all`:

```bash
rela export --all --format json
```

Output structure:

```json
{
  "entities": [
    {"id": "CTRL-001", "type": "control", "properties": {...}},
    {"id": "RISK-001", "type": "risk", "properties": {...}}
  ],
  "relations": [
    {"from": "CTRL-001", "relation": "mitigates", "to": "RISK-001"}
  ]
}
```

Note: `--all` with `--format csv` is not supported (use JSON or YAML).

## Practical Examples

### ISO 27001 Statement of Applicability (SoA)

Generate a Statement of Applicability report:

```bash
rela export control --with-relations --format json | \
  jq -r '["Control","Title","Applicable","Status","Evidence Count"],
         (.[] | select(.properties.iso27001 != null) |
           [.properties.iso27001,
            .properties.title,
            .properties.applicability // "applicable",
            .properties.status,
            (.relations.outgoing.evidencedBy | length | tostring)])
         | @csv' > soa.csv
```

### Evidence Gap Report

Find controls missing evidence:

```bash
rela export control --with-relations --format json | \
  jq '.[] |
      select(.relations == null or .relations.outgoing == null or .relations.outgoing.evidencedBy == null) |
      {id, title: .properties.title, iso27001: .properties.iso27001}'
```

### Risk Treatment Report

Generate a risk treatment status report:

```bash
rela export risk --with-relations --format json | \
  jq '.[] | {
    id,
    title: .properties.title,
    severity: .properties.severity,
    treatment_status: .properties.treatment_status,
    controls: [.relations.incoming.mitigates[]?.id] | join(", ")
  }'
```

### Draft Items Report

Find all items still in draft status:

```bash
rela export --all --format json | \
  jq '.entities[] | select(.properties.status == "draft") | {type, id, title: .properties.title}'
```

### Using Miller for CSV Analysis

[Miller](https://miller.readthedocs.io/) is excellent for CSV processing:

```bash
# Filter applicable controls
rela export control --format csv | \
  mlr --csv filter '$applicability == "applicable"' then sort -f iso27001

# Count by status
rela export control --format csv | \
  mlr --csv stats1 -a count -g status

# Select specific columns
rela export control --format csv | \
  mlr --csv cut -f id,title,status
```

### Using DuckDB for SQL Queries

[DuckDB](https://duckdb.org/) can query JSON and CSV directly:

```bash
# Export to JSON file
rela export control --format json > controls.json

# Query with DuckDB
duckdb -c "
  SELECT id, properties->>'title' as title, properties->>'status' as status
  FROM read_json_auto('controls.json')
  WHERE properties->>'status' = 'draft'
"
```

### Using Python/pandas

```python
import json
import subprocess
import pandas as pd

# Get export data
result = subprocess.run(
    ['rela', 'export', 'control', '--format', 'json'],
    capture_output=True, text=True
)
data = json.loads(result.stdout)

# Convert to DataFrame
df = pd.json_normalize(data)

# Analyze
print(df.groupby('properties.status').size())
```

### CI/CD Integration

Add compliance checks to your pipeline:

```bash
#!/bin/bash
# check-compliance.sh

# Check for controls without evidence
GAPS=$(rela export control --with-relations --format json | \
  jq '[.[] | select(.relations.outgoing.evidencedBy == null)] | length')

if [ "$GAPS" -gt 0 ]; then
  echo "Warning: $GAPS controls have no evidence attached"
  rela export control --with-relations --format json | \
    jq '.[] | select(.relations.outgoing.evidencedBy == null) | .id'
  exit 1
fi

# Check for high-severity untreated risks
UNTREATED=$(rela export risk --format json | \
  jq '[.[] | select(.properties.severity == "high" and .properties.treatment_status != "treated")] | length')

if [ "$UNTREATED" -gt 0 ]; then
  echo "Error: $UNTREATED high-severity risks are not treated"
  exit 1
fi

echo "All compliance checks passed"
```

## Importing Data

The `rela import` command is the inverse of export, allowing you to bulk-create entities and
relations from JSON, YAML, or CSV files.

### Round-Trip: Backup and Restore

```bash
# Export everything to a backup file
rela export --all --format json > backup.json

# Later, restore to a new project
rela init
rela import backup.json
```

### Bulk Creation from Spreadsheet

1. Prepare your data in a spreadsheet with columns: `id`, `type`, `title`, `status`, etc.
2. Export to CSV
3. Import:

```bash
rela import entities.csv
```

### Migration from Other Tools

Convert your existing data to rela's JSON format:

```json
{
  "entities": [
    {
      "id": "REQ-001",
      "type": "requirement",
      "properties": { "title": "...", "status": "draft" }
    }
  ],
  "relations": [{ "from": "DEC-001", "relation": "addresses", "to": "REQ-001" }]
}
```

Then import:

```bash
# Validate first
rela import --dry-run migration.json

# Import
rela import migration.json
```

### Import Options

| Flag            | Description                     |
| --------------- | ------------------------------- |
| `--dry-run`     | Validate without creating files |
| `--update`      | Replace existing entities       |
| `--skip-errors` | Continue on validation errors   |
| `--relations`   | Separate relations CSV file     |

See [CLI Reference](cli-reference.md#rela-import) for full details.

---

## Tips

1. **Use `--format json` with `jq`** for maximum flexibility in filtering and transforming data.

2. **Use `--format csv` with `mlr`** for quick tabular analysis and spreadsheet export.

3. **Pipe to file** when working with large datasets:

   ```bash
   rela export --all --format json > full-export.json
   ```

4. **Combine with `watch`** for live monitoring:

   ```bash
   watch -n 5 'rela export control --format json | jq "[.[] | select(.properties.status == \"draft\")] | length"'
   ```

5. **Use relation data** for traceability reports:

   ```bash
   # Controls with the most risks mitigated
   rela export control --with-relations --format json | \
     jq 'sort_by(.relations.outgoing.mitigates | length) | reverse | .[:5] | .[].id'
   ```

6. **Use export/import for project backup**:

   ```bash
   rela export --all -f json > "backup-$(date +%Y%m%d).json"
   ```
