# Vue.js Migration Plan

## Overview

Migrate the data entry web app from HTMX + server-rendered templates to a Vue.js
SPA backed by a generic REST API. The API is dynamically composed from the
metamodel definitions, providing type-safe endpoints for all entity types.

## Goals

1. **Clean REST API** - Resource-oriented, metamodel-driven endpoints
2. **Modern frontend** - Vue 3 + Composition API + TypeScript
3. **Config-driven UI** - Forms, lists, views defined in YAML, rendered
   client-side
4. **Incremental migration** - Run alongside HTMX during transition

---

## API Design

### URL Structure

The API uses the metamodel to dynamically generate endpoints per entity type:

```text
/api/v1/{type}          # Collection (list, create)
/api/v1/{type}/{id}     # Resource (get, update, delete)
```

**Examples** (given metamodel defines `ticket`, `feature`, `decision`):

| Method | URL                         | Description       |
| ------ | --------------------------- | ----------------- |
| GET    | `/api/v1/tickets`           | List all tickets  |
| POST   | `/api/v1/tickets`           | Create ticket     |
| GET    | `/api/v1/tickets/TKT-001`   | Get ticket        |
| PATCH  | `/api/v1/tickets/TKT-001`   | Update ticket     |
| DELETE | `/api/v1/tickets/TKT-001`   | Delete ticket     |
| GET    | `/api/v1/features`          | List all features |
| GET    | `/api/v1/decisions/DEC-005` | Get decision      |

### Pluralization

Rela already has pluralization via `EntityDef.GetDirPlural(typeName)`:

- Default: append `s` to type name (`ticket` → `tickets`)
- Override: set `plural` field in metamodel

```yaml
entities:
  # Default: "tickets"
  ticket:
    properties: ...

  # Override for irregular plurals
  policy:
    plural: policies
    properties: ...

  # Override for collective nouns
  feedback:
    plural: feedback
    properties: ...
```

The API uses the same logic:

```go
for typeName, entityDef := range a.meta.Entities {
    plural := entityDef.GetDirPlural(typeName)  // Uses existing method
    r.Route("/"+plural, func(r chi.Router) {
        // ...
    })
}
```

### Relations as Sub-resources

Relations can be accessed as nested resources:

```text
/api/v1/{type}/{id}/relations                    # All relations for entity
/api/v1/{type}/{id}/relations/{relation-type}    # Relations of specific type
```

**Examples**:

| Method | URL                                                    | Description                     |
| ------ | ------------------------------------------------------ | ------------------------------- |
| GET    | `/api/v1/tickets/TKT-001/relations`                    | All relations for TKT-001       |
| GET    | `/api/v1/tickets/TKT-001/relations/implements`         | Features this ticket implements |
| POST   | `/api/v1/tickets/TKT-001/relations/implements`         | Link ticket to feature          |
| DELETE | `/api/v1/tickets/TKT-001/relations/implements/FTR-002` | Unlink                          |

### Query Parameters

Standard filtering, sorting, pagination:

```text
GET /api/v1/tickets?status=open&priority=high     # Filter
GET /api/v1/tickets?sort=-created,title           # Sort (- = desc)
GET /api/v1/tickets?page=2&per_page=25            # Pagination (offset-based)
GET /api/v1/tickets?cursor=eyJpZCI6MTAwfQ         # Pagination (cursor-based)
GET /api/v1/tickets?include=implements,affects    # Embed related entities
GET /api/v1/tickets?fields=id,title,status        # Sparse fieldsets
```

### REST Standards

The API follows these established standards:

#### RFC 5988: Link Headers (Pagination)

```http
HTTP/1.1 200 OK
Link: </api/v1/tickets?page=1>; rel="first",
      </api/v1/tickets?page=2>; rel="prev",
      </api/v1/tickets?page=4>; rel="next",
      </api/v1/tickets?page=10>; rel="last"
X-Total-Count: 247
X-Page: 3
X-Per-Page: 25
```

Clients parse `Link` header for navigation without hardcoding URL construction.

#### RFC 7807: Problem Details (Errors)

```json
{
  "type": "https://rela.dev/errors/validation-failed",
  "title": "Validation Failed",
  "status": 422,
  "detail": "The entity could not be saved due to validation errors",
  "instance": "/api/v1/tickets",
  "errors": [
    {
      "field": "status",
      "message": "must be one of: draft, open, done"
    },
    {
      "field": "title",
      "message": "is required"
    }
  ]
}
```

Content-Type: `application/problem+json`

#### ETags + Conditional Requests (Caching)

```http
# Initial request
GET /api/v1/tickets/TKT-001
HTTP/1.1 200 OK
ETag: "a1b2c3d4"
Cache-Control: private, max-age=0, must-revalidate

# Subsequent request
GET /api/v1/tickets/TKT-001
If-None-Match: "a1b2c3d4"
HTTP/1.1 304 Not Modified

# Optimistic locking on update
PATCH /api/v1/tickets/TKT-001
If-Match: "a1b2c3d4"
HTTP/1.1 412 Precondition Failed  # If entity changed
```

ETags computed from entity content hash (properties + content + relations).

#### Cursor-Based Pagination (Large Datasets)

Offset pagination breaks with large datasets. Cursor pagination is stable:

```http
GET /api/v1/tickets?per_page=25
HTTP/1.1 200 OK
Link: </api/v1/tickets?cursor=eyJpZCI6IlRLVC0wMjUifQ&per_page=25>; rel="next"

{
  "data": [...],
  "meta": {
    "per_page": 25,
    "has_more": true,
    "next_cursor": "eyJpZCI6IlRLVC0wMjUifQ"
  }
}
```

Cursor is base64-encoded `{"id": "TKT-025", "sort": [...]}`.

Support both modes:

- `?page=N` - offset-based (simple, but unstable for large sets)
- `?cursor=X` - cursor-based (stable, recommended for >1000 items)

#### Sparse Fieldsets (Bandwidth)

Request only needed fields:

```http
GET /api/v1/tickets?fields=id,title,status

{
  "data": [
    {"id": "TKT-001", "title": "Fix bug", "status": "open"},
    {"id": "TKT-002", "title": "Add feature", "status": "draft"}
  ]
}
```

Nested fields: `?fields=id,title,relations.implements`

#### Include/Expand (N+1 Prevention)

Embed related entities in response:

```http
GET /api/v1/tickets/TKT-001?include=implements,affects

{
  "id": "TKT-001",
  "type": "ticket",
  "properties": {...},
  "relations": {
    "implements": ["FTR-001"],
    "affects": ["CONCEPT-AUTH"]
  },
  "included": {
    "FTR-001": {
      "id": "FTR-001",
      "type": "feature",
      "properties": {"title": "User Authentication", "status": "planned"}
    },
    "CONCEPT-AUTH": {
      "id": "CONCEPT-AUTH",
      "type": "concept",
      "properties": {"title": "Authentication System"}
    }
  }
}
```

#### JSON:API Ideas

Adopting useful patterns from JSON:API spec:

**1. Nested includes (graph traversal)**

Dot notation to traverse relationships:

```http
# Ticket → implements → requires (2 hops)
GET /api/v1/tickets/TKT-001?include=implements.requires

# Ticket → all relations → their relations (2 hops, all types)
GET /api/v1/tickets/TKT-001?include=*.*

{
  "id": "TKT-001",
  "relations": {
    "implements": ["FTR-001"]
  },
  "included": {
    "FTR-001": {
      "id": "FTR-001",
      "type": "feature",
      "relations": {"requires": ["CONCEPT-AUTH"]}
    },
    "CONCEPT-AUTH": {
      "id": "CONCEPT-AUTH",
      "type": "concept",
      "properties": {"title": "Authentication"}
    }
  }
}
```

This is essentially `trace_from` exposed via REST.

**2. Namespaced filters**

Clearer filter syntax, avoids collision with other params:

```http
# Instead of: ?status=open&priority=high
GET /api/v1/tickets?filter[status]=open&filter[priority]=high

# Operators in brackets
GET /api/v1/tickets?filter[created][gte]=2024-01-01
GET /api/v1/tickets?filter[title][contains]=auth
GET /api/v1/tickets?filter[priority][in]=high,critical

# Filter on relations
GET /api/v1/tickets?filter[implements]=FTR-001
GET /api/v1/tickets?filter[implements][any]=true  # Has any implements relation
```

**3. Error source pointers**

Pinpoint exact location of validation errors:

```json
{
  "type": "https://rela.dev/errors/validation-failed",
  "title": "Validation Failed",
  "status": 422,
  "errors": [
    {
      "source": {"pointer": "/properties/status"},
      "code": "invalid_enum",
      "detail": "must be one of: draft, open, done"
    },
    {
      "source": {"pointer": "/relations/implements"},
      "code": "min_cardinality",
      "detail": "requires at least 1 relation"
    },
    {
      "source": {"pointer": "/content"},
      "code": "missing_header",
      "detail": "missing required header: ## Context"
    }
  ]
}
```

The `pointer` uses JSON Pointer syntax (RFC 6901), making it easy for clients
to highlight the exact field.

**4. Relationship metadata**

Relations can carry properties (rela already supports this):

```json
{
  "id": "TKT-001",
  "relations": {
    "implements": [
      {
        "id": "FTR-001",
        "meta": {
          "created": "2024-03-01",
          "weight": "high",
          "notes": "Core dependency"
        }
      }
    ]
  }
}
```

Opt-in via `?include=relations.meta` to avoid bloat.

**5. Document-level meta**

Useful stats and debug info:

```json
{
  "data": [...],
  "meta": {
    "total": 247,
    "filtered": 42,
    "page": 1,
    "per_page": 25,
    "query_time_ms": 12,
    "cache_hit": true,
    "warnings": [
      {"code": "deprecated_filter", "message": "status=active is deprecated, use status=open"}
    ]
  }
}
```

**What we skip from JSON:API:**

- Strict resource object structure (`attributes` vs `relationships` separation)
- `jsonapi` version object in every response
- Required `type` in request bodies (we infer from URL)
- `self` links on every relationship (use `_actions` opt-in instead)
- Full link objects with `href`/`meta` (overkill)

#### Prefer Header (Response Control)

```http
# Return full entity after create/update
POST /api/v1/tickets
Prefer: return=representation
HTTP/1.1 201 Created
{...full entity...}

# Return minimal response (just ID)
POST /api/v1/tickets
Prefer: return=minimal
HTTP/1.1 201 Created
{"id": "TKT-042"}

# Count only (no data)
GET /api/v1/tickets?status=open
Prefer: return=count
HTTP/1.1 200 OK
{"count": 42}
```

#### CORS Headers

```http
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization, If-Match, If-None-Match
Access-Control-Expose-Headers: Link, ETag, X-Total-Count, X-Page, X-Per-Page
```

#### Content Negotiation

```http
# Default JSON
GET /api/v1/tickets
Accept: application/json

# CSV export
GET /api/v1/tickets
Accept: text/csv

# YAML export
GET /api/v1/tickets
Accept: application/yaml
```

#### HATEOAS (Selective)

Full HATEOAS adds complexity without much benefit when clients have the schema.
Rela uses **schema-as-contract** instead: the metamodel defines all valid
operations, and clients load it once via `GET /_schema`.

**Useful HATEOAS elements we adopt:**

1. **Pagination links** (RFC 5988 Link header) - already included above
2. **Self links** - useful for embedded/included resources
3. **State-dependent actions** - show what's actually possible

```json
{
  "id": "TKT-001",
  "type": "ticket",
  "properties": {"title": "Fix bug", "status": "open"},
  "_self": "/api/v1/tickets/TKT-001",
  "_actions": {
    "delete": {
      "allowed": false,
      "reason": "Entity has 3 incoming relations"
    },
    "transitions": ["in-progress", "blocked", "done"]
  }
}
```

**`_actions` provides runtime constraints** that the static schema can't express:

- Can't delete if entity has dependents (graph constraint)
- Valid status transitions from current state (state machine)
- Permission-based restrictions (future auth)

**What we skip:**

- Links to related resources (client constructs from `relations` + schema)
- Links to collection (client knows the pattern)
- Full HAL/JSON-API `_links` structure (over-engineering)

The `_actions` field is **opt-in** via query param:

```http
GET /api/v1/tickets/TKT-001?include=_actions
```

### Response Format

**Single entity**:

```json
{
  "id": "TKT-001",
  "type": "ticket",
  "properties": {
    "title": "Fix login bug",
    "status": "open",
    "priority": "high"
  },
  "content": "Markdown body...",
  "relations": {
    "implements": ["FTR-001"],
    "affects": ["CONCEPT-AUTH"]
  }
}
```

**Collection**:

```json
{
  "data": [...],
  "meta": {
    "total": 42,
    "page": 1,
    "per_page": 25
  }
}
```

**Errors**:

```json
{
  "error": "validation_failed",
  "message": "Invalid entity",
  "details": [
    {"field": "status", "message": "must be one of: draft, open, done"}
  ]
}
```

### System Endpoints (Underscore Prefix)

All non-entity endpoints use `_` prefix to avoid conflicts with user-defined
entity types:

| Endpoint                           | Description                                          |
| ---------------------------------- | ---------------------------------------------------- |
| `GET /api/v1/_schema`              | Full metamodel (entity types, relations, properties) |
| `GET /api/v1/_schema/types`        | List of entity type names                            |
| `GET /api/v1/_schema/types/{type}` | Schema for specific type                             |
| `GET /api/v1/_schema/relations`    | Relation type definitions                            |
| `GET /api/v1/_config`              | UI config (forms, lists, views, navigation)          |
| `GET /api/v1/_config/forms/{id}`   | Specific form config                                 |
| `GET /api/v1/_config/lists/{id}`   | Specific list config                                 |
| `GET /api/v1/_analyze`             | Run all analysis checks                              |
| `GET /api/v1/_analyze/{check}`     | Run specific check                                   |
| `GET /api/v1/_search?q=...`        | Full-text search                                     |
| `GET /api/v1/_events`              | SSE for live updates                                 |
| `GET /api/v1/_git/status`          | Git status                                           |
| `POST /api/v1/_git/sync`           | Git sync                                             |
| `POST /api/v1/_commands/{id}`      | Execute command (SSE response)                       |

**Why underscore?**

- Cannot conflict with entity types (identifiers can't start with `_`)
- URL-safe, no encoding needed
- Common convention for meta/system endpoints
- Visually distinct from entity routes

### Entity Actions

Actions on specific entities use `_actions` sub-path:

```text
POST /api/v1/tickets/TKT-001/_actions/clone    # Clone entity
POST /api/v1/tickets/TKT-001/_actions/archive  # Archive entity
```

---

## Implementation: Go Backend

### Router Structure

```go
// internal/dataentry/router_api.go

func (a *App) registerAPIRoutes(r chi.Router) {
    r.Route("/api/v1", func(r chi.Router) {
        // System endpoints (underscore prefix to avoid entity type conflicts)
        r.Route("/_schema", func(r chi.Router) {
            r.Get("/", a.handleSchema)
            r.Get("/types", a.handleSchemaTypes)
            r.Get("/types/{type}", a.handleSchemaType)
            r.Get("/relations", a.handleSchemaRelations)
        })
        r.Route("/_config", func(r chi.Router) {
            r.Get("/", a.handleConfig)
            r.Get("/forms/{id}", a.handleConfigForm)
            r.Get("/lists/{id}", a.handleConfigList)
        })
        r.Get("/_search", a.handleSearch)
        r.Get("/_analyze", a.handleAnalyze)
        r.Get("/_analyze/{check}", a.handleAnalyzeCheck)
        r.Route("/_git", func(r chi.Router) {
            r.Get("/status", a.handleGitStatus)
            r.Post("/sync", a.handleGitSync)
        })
        r.Get("/_events", a.handleSSE)
        r.Post("/_commands/{id}", a.handleCommand)

        // Dynamic entity routes (registered per type from metamodel)
        for typeName, entityDef := range a.meta.Entities {
            plural := entityDef.GetDirPlural(typeName)
            r.Route("/"+plural, func(r chi.Router) {
                r.Get("/", a.handleListEntities(typeName))
                r.Post("/", a.handleCreateEntity(typeName))
                r.Route("/{id}", func(r chi.Router) {
                    r.Get("/", a.handleGetEntity(typeName))
                    r.Patch("/", a.handleUpdateEntity(typeName))
                    r.Delete("/", a.handleDeleteEntity(typeName))
                    r.Get("/relations", a.handleEntityRelations(typeName))
                    r.Route("/relations/{relType}", func(r chi.Router) {
                        r.Get("/", a.handleEntityRelationsOfType(typeName))
                        r.Post("/", a.handleCreateRelation(typeName))
                        r.Delete("/{targetId}", a.handleDeleteRelation(typeName))
                    })
                    r.Post("/_actions/{action}", a.handleEntityAction(typeName))
                })
            })
        }
    })
}
```

### Handler Pattern

```go
// Returns a handler bound to specific entity type
func (a *App) handleListEntities(typeName string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        a.mu.RLock()
        defer a.mu.RUnlock()

        // Get entity type schema
        entityType := a.meta.Entities[typeName]

        // Query graph
        entities := a.g.NodesByType(typeName)

        // Apply filters from query params
        entities = a.applyFilters(entities, r.URL.Query(), entityType)

        // Apply sorting
        entities = a.applySorting(entities, r.URL.Query())

        // Paginate
        entities, meta := a.paginate(entities, r.URL.Query())

        // Serialize
        result := APIListResponse{
            Data: a.serializeEntities(entities),
            Meta: meta,
        }

        json.NewEncoder(w).Encode(result)
    }
}
```

### Type-safe Validation

```go
func (a *App) validateEntity(typeName string, data map[string]any) []ValidationError {
    entityType := a.meta.Entities[typeName]
    var errors []ValidationError

    for propName, propDef := range entityType.Properties {
        value := data[propName]

        if propDef.Required && value == nil {
            errors = append(errors, ValidationError{
                Field:   propName,
                Message: "required",
            })
            continue
        }

        // Type validation based on metamodel
        switch propDef.Type {
        case "enum":
            if !slices.Contains(propDef.Values, value.(string)) {
                errors = append(errors, ValidationError{
                    Field:   propName,
                    Message: fmt.Sprintf("must be one of: %s", strings.Join(propDef.Values, ", ")),
                })
            }
        case "date":
            // validate date format
        // ...
        }
    }

    return errors
}
```

---

## Implementation: Vue Frontend

### Project Structure

```text
frontend/
├── src/
│   ├── api/
│   │   ├── client.ts          # Axios/fetch wrapper
│   │   ├── entities.ts        # Entity CRUD operations
│   │   ├── relations.ts       # Relation operations
│   │   └── schema.ts          # Schema/config fetching
│   │
│   ├── stores/
│   │   ├── schema.ts          # Metamodel + UI config
│   │   ├── entities.ts        # Entity cache
│   │   └── ui.ts              # UI state (sidebar, modals)
│   │
│   ├── composables/
│   │   ├── useEntity.ts       # Entity CRUD logic
│   │   ├── useList.ts         # List filtering/sorting/pagination
│   │   ├── useForm.ts         # Form state + validation
│   │   └── useKeyboard.ts     # Keyboard shortcuts
│   │
│   ├── components/
│   │   ├── forms/
│   │   │   ├── DynamicForm.vue
│   │   │   ├── FieldRenderer.vue
│   │   │   ├── TextInput.vue
│   │   │   ├── SelectInput.vue
│   │   │   ├── MultiSelect.vue
│   │   │   ├── DatePicker.vue
│   │   │   ├── MarkdownEditor.vue
│   │   │   └── RelationPicker.vue
│   │   │
│   │   ├── lists/
│   │   │   ├── EntityList.vue
│   │   │   ├── FilterBar.vue
│   │   │   ├── SortableColumn.vue
│   │   │   └── Pagination.vue
│   │   │
│   │   ├── views/
│   │   │   ├── CustomView.vue
│   │   │   ├── SectionRenderer.vue
│   │   │   └── CardGrid.vue
│   │   │
│   │   ├── kanban/
│   │   │   ├── KanbanBoard.vue
│   │   │   ├── KanbanColumn.vue
│   │   │   └── KanbanCard.vue
│   │   │
│   │   ├── graph/
│   │   │   └── GraphView.vue
│   │   │
│   │   └── common/
│   │       ├── Sidebar.vue
│   │       ├── Modal.vue
│   │       ├── Toast.vue
│   │       ├── Badge.vue
│   │       └── MermaidDiagram.vue
│   │
│   ├── views/
│   │   ├── ListView.vue
│   │   ├── FormView.vue
│   │   ├── EntityView.vue
│   │   ├── DashboardView.vue
│   │   ├── SearchView.vue
│   │   ├── SettingsView.vue
│   │   └── AnalyzeView.vue
│   │
│   ├── router/
│   │   └── index.ts
│   │
│   ├── types/
│   │   ├── entity.ts
│   │   ├── schema.ts
│   │   └── config.ts
│   │
│   ├── App.vue
│   └── main.ts
│
├── index.html
├── vite.config.ts
├── tsconfig.json
└── package.json
```

### API Client

```typescript
// src/api/client.ts
import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  headers: { 'Content-Type': 'application/json' }
})

// src/api/entities.ts
import { useSchemaStore } from '@/stores/schema'

// Get plural form from schema (includes metamodel overrides)
function getPlural(type: string): string {
  const schema = useSchemaStore()
  const entityType = schema.entityTypes.get(type)
  return entityType?.plural ?? type + 's'
}

export async function listEntities(
  type: string,
  params?: ListParams
): Promise<ListResponse<Entity>> {
  const { data } = await api.get(`/${getPlural(type)}`, { params })
  return data
}

export async function getEntity(type: string, id: string): Promise<Entity> {
  const { data } = await api.get(`/${getPlural(type)}/${id}`)
  return data
}

export async function createEntity(type: string, entity: CreateEntity): Promise<Entity> {
  const { data } = await api.post(`/${getPlural(type)}`, entity)
  return data
}

export async function updateEntity(
  type: string,
  id: string,
  patch: Partial<Entity>
): Promise<Entity> {
  const { data } = await api.patch(`/${getPlural(type)}/${id}`, patch)
  return data
}

export async function deleteEntity(type: string, id: string): Promise<void> {
  await api.delete(`/${getPlural(type)}/${id}`)
}
```

**Schema response includes plural forms:**

```json
{
  "entities": {
    "ticket": {
      "label": "Ticket",
      "plural": "tickets",
      "properties": { ... }
    },
    "policy": {
      "label": "Policy",
      "plural": "policies",
      "properties": { ... }
    }
  }
}
```

### Schema Store

```typescript
// src/stores/schema.ts
import { defineStore } from 'pinia'
import { getSchema, getConfig } from '@/api/schema'

interface SchemaState {
  entityTypes: Map<string, EntityType>
  relationTypes: Map<string, RelationType>
  forms: Map<string, FormConfig>
  lists: Map<string, ListConfig>
  views: Map<string, ViewConfig>
  navigation: NavigationEntry[]
  loaded: boolean
}

export const useSchemaStore = defineStore('schema', {
  state: (): SchemaState => ({
    entityTypes: new Map(),
    relationTypes: new Map(),
    forms: new Map(),
    lists: new Map(),
    views: new Map(),
    navigation: [],
    loaded: false,
  }),

  getters: {
    getEntityType: (state) => (name: string) => state.entityTypes.get(name),
    getForm: (state) => (id: string) => state.forms.get(id),
    getList: (state) => (id: string) => state.lists.get(id),
  },

  actions: {
    async load() {
      const [schema, config] = await Promise.all([
        getSchema(),
        getConfig(),
      ])

      this.entityTypes = new Map(Object.entries(schema.entities))
      this.relationTypes = new Map(Object.entries(schema.relations))
      this.forms = new Map(Object.entries(config.forms))
      this.lists = new Map(Object.entries(config.lists))
      this.views = new Map(Object.entries(config.views))
      this.navigation = config.navigation
      this.loaded = true
    }
  }
})
```

### Dynamic Form Component

```vue
<!-- src/components/forms/DynamicForm.vue -->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useSchemaStore } from '@/stores/schema'
import FieldRenderer from './FieldRenderer.vue'
import RelationPicker from './RelationPicker.vue'

const props = defineProps<{
  formId: string
  entityId?: string  // undefined = create mode
  initialData?: Entity
}>()

const emit = defineEmits<{
  submit: [data: FormData]
  cancel: []
}>()

const schema = useSchemaStore()
const formConfig = computed(() => schema.getForm(props.formId))
const entityType = computed(() => schema.getEntityType(formConfig.value?.type))

const formData = ref<Record<string, any>>({})
const relations = ref<Record<string, string[]>>({})
const errors = ref<Record<string, string>>({})
const isDirty = ref(false)

// Initialize form data
watch(() => props.initialData, (data) => {
  if (data) {
    formData.value = { ...data.properties }
    relations.value = { ...data.relations }
  } else {
    // Apply defaults from config
    formConfig.value?.fields.forEach(field => {
      if (field.default !== undefined) {
        formData.value[field.property] = field.default
      }
    })
  }
}, { immediate: true })

// Track dirty state
watch(formData, () => { isDirty.value = true }, { deep: true })

async function handleSubmit() {
  errors.value = {}

  // Client-side validation
  const validationErrors = validate(formData.value, entityType.value)
  if (validationErrors.length > 0) {
    validationErrors.forEach(e => { errors.value[e.field] = e.message })
    return
  }

  emit('submit', {
    properties: formData.value,
    relations: relations.value,
  })
}
</script>

<template>
  <form @submit.prevent="handleSubmit" class="dynamic-form">
    <div class="form-fields">
      <template v-for="field in formConfig?.fields" :key="field.property">
        <FieldRenderer
          :field="field"
          :schema="entityType?.properties[field.property]"
          v-model="formData[field.property]"
          :error="errors[field.property]"
        />
      </template>
    </div>

    <div class="form-relations" v-if="formConfig?.relations">
      <template v-for="rel in formConfig.relations" :key="rel.type">
        <RelationPicker
          :config="rel"
          :relation-type="schema.relationTypes.get(rel.type)"
          v-model="relations[rel.type]"
        />
      </template>
    </div>

    <div class="form-actions">
      <button type="button" @click="emit('cancel')">Cancel</button>
      <button type="submit" :disabled="!isDirty">
        {{ entityId ? 'Update' : 'Create' }}
      </button>
    </div>
  </form>
</template>
```

### Router Configuration

```typescript
// src/router/index.ts
import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: () => import('@/views/DashboardView.vue') },
  { path: '/list/:id', component: () => import('@/views/ListView.vue') },
  { path: '/form/:formId', component: () => import('@/views/FormView.vue') },
  { path: '/form/:formId/:entityId', component: () => import('@/views/FormView.vue') },
  { path: '/entity/:type/:id', component: () => import('@/views/EntityView.vue') },
  { path: '/view/:viewId/:entityId', component: () => import('@/views/CustomView.vue') },
  { path: '/kanban/:id', component: () => import('@/views/KanbanView.vue') },
  { path: '/search', component: () => import('@/views/SearchView.vue') },
  { path: '/analyze', component: () => import('@/views/AnalyzeView.vue') },
  { path: '/settings', component: () => import('@/views/SettingsView.vue') },
  { path: '/graph', component: () => import('@/views/GraphView.vue') },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
```

---

## Migration Strategy

### Parallel v1/v2 Architecture

During migration, both versions run simultaneously in the same server instance:

```text
/           → v1 (HTMX)      # Original app
/v2/        → v2 (Vue)       # New app under development
/api/       → v1 API         # Existing JSON endpoints
/api/v1/    → v2 API         # New REST API
```

This allows direct comparison of features side-by-side. Open two browser tabs:

- `http://localhost:8080/list/tickets` (v1)
- `http://localhost:8080/v2/list/tickets` (v2)

### Puppeteer E2E Testing

Each feature must pass Puppeteer tests before considered complete. Tests run
against both v1 and v2 to verify parity.

```text
frontend/
├── e2e/
│   ├── fixtures/           # Test data setup
│   ├── helpers/
│   │   ├── v1.ts           # v1 page helpers (HTMX selectors)
│   │   └── v2.ts           # v2 page helpers (Vue selectors)
│   ├── specs/
│   │   ├── list.spec.ts
│   │   ├── form.spec.ts
│   │   ├── entity.spec.ts
│   │   ├── search.spec.ts
│   │   ├── kanban.spec.ts
│   │   ├── dashboard.spec.ts
│   │   ├── settings.spec.ts
│   │   └── ...
│   └── parity-report.ts    # Compare v1 vs v2 results
```

**Test pattern** - each spec tests both versions:

```typescript
// e2e/specs/list.spec.ts
import { test, expect } from '@playwright/test'
import { V1ListPage } from '../helpers/v1'
import { V2ListPage } from '../helpers/v2'

const versions = [
  { name: 'v1', Page: V1ListPage, baseUrl: '' },
  { name: 'v2', Page: V2ListPage, baseUrl: '/v2' },
]

for (const { name, Page, baseUrl } of versions) {
  test.describe(`List View (${name})`, () => {
    test('displays entities in table', async ({ page }) => {
      const listPage = new Page(page, baseUrl)
      await listPage.goto('/list/tickets')

      const rows = await listPage.getTableRows()
      expect(rows.length).toBeGreaterThan(0)
    })

    test('filters by status', async ({ page }) => {
      const listPage = new Page(page, baseUrl)
      await listPage.goto('/list/tickets')
      await listPage.selectFilter('status', 'open')

      const rows = await listPage.getTableRows()
      for (const row of rows) {
        expect(await row.getStatus()).toBe('open')
      }
    })

    test('sorts by column', async ({ page }) => {
      const listPage = new Page(page, baseUrl)
      await listPage.goto('/list/tickets')
      await listPage.clickColumnHeader('title')

      const titles = await listPage.getColumnValues('title')
      expect(titles).toEqual([...titles].sort())
    })

    test('pagination works', async ({ page }) => {
      const listPage = new Page(page, baseUrl)
      await listPage.goto('/list/tickets')

      const page1Items = await listPage.getTableRows()
      await listPage.nextPage()
      const page2Items = await listPage.getTableRows()

      expect(page1Items[0]).not.toEqual(page2Items[0])
    })
  })
}
```

**Parity report** - automated comparison:

```typescript
// e2e/parity-report.ts
// Generates report showing which features pass in v1 vs v2

interface ParityResult {
  feature: string
  v1: 'pass' | 'fail' | 'skip'
  v2: 'pass' | 'fail' | 'skip'
  parity: boolean
}

// Output:
// ┌─────────────────────┬──────┬──────┬────────┐
// │ Feature             │ v1   │ v2   │ Parity │
// ├─────────────────────┼──────┼──────┼────────┤
// │ list.display        │ pass │ pass │ ✓      │
// │ list.filter         │ pass │ pass │ ✓      │
// │ list.sort           │ pass │ fail │ ✗      │
// │ form.create         │ pass │ pass │ ✓      │
// │ form.validation     │ pass │ skip │ ✗      │
// │ kanban.drag         │ pass │ skip │ ✗      │
// └─────────────────────┴──────┴──────┴────────┘
```

---

### Phase 1: API Foundation

**Tasks:**

1. Create `/api/v1/` router structure
2. Implement `GET /api/v1/schema` (metamodel)
3. Implement `GET /api/v1/config` (UI config)
4. Implement dynamic entity endpoints per type
5. Implement relation sub-resources
6. Add query param filtering/sorting/pagination
7. Write Go unit tests for all endpoints
8. Generate OpenAPI spec

**Puppeteer tests:** None yet (API only)

**Done when:** All API endpoints return correct JSON, unit tests pass

---

### Phase 2: Vue Scaffold + Navigation

**Tasks:**

1. Initialize Vue 3 + Vite + TypeScript project
2. Configure build output to `internal/dataentry/static/v2/`
3. Set up Go routing for `/v2/*` → Vue SPA
4. Implement API client (`/api/v1/` calls)
5. Create Pinia stores (schema, entities, ui)
6. Build layout shell (sidebar + content area)
7. Implement navigation from config
8. Add version switcher link (v1 ↔ v2)

**Puppeteer tests:**

- [ ] Navigation renders sidebar items from config
- [ ] Clicking nav item changes route
- [ ] Version switcher links work

**Done when:** Can navigate between empty pages, sidebar matches v1

---

### Phase 3: Entity Lists

**Tasks:**

1. `EntityList.vue` - table with columns from config
2. `FilterBar.vue` - dynamic filters per list config
3. `SortableColumn.vue` - click to sort
4. `Pagination.vue` - page controls
5. Badge styling for enum properties
6. Link to entity detail/form

**Puppeteer tests:**

- [ ] List displays correct columns
- [ ] Filter by each property type works
- [ ] Sort ascending/descending works
- [ ] Pagination navigates correctly
- [ ] Row click navigates to entity

**Done when:** All list Puppeteer tests pass for both v1 and v2

---

### Phase 4: Forms (Create/Edit)

**Tasks:**

1. `DynamicForm.vue` - renders fields from config
2. Field widgets: text, textarea, select, multi-select, checkbox, date, number
3. `RelationPicker.vue` - select related entities
4. Client-side validation (from metamodel)
5. Server-side validation error display
6. Create mode with defaults
7. Edit mode with pre-populated data
8. Dirty state tracking + unsaved changes warning

**Puppeteer tests:**

- [ ] Form renders all configured fields
- [ ] Each widget type works correctly
- [ ] Validation errors display inline
- [ ] Create saves and redirects
- [ ] Edit loads existing data
- [ ] Unsaved changes prompt on navigate

**Done when:** All form Puppeteer tests pass for both v1 and v2

---

### Phase 5: Entity Detail View

**Tasks:**

1. `EntityDetail.vue` - properties grid
2. Relations display with links
3. Markdown content rendering
4. Checkbox toggle in content
5. Edit/delete buttons
6. Scope navigation (prev/next)

**Puppeteer tests:**

- [ ] Properties display correctly
- [ ] Relations link to targets
- [ ] Markdown renders (headers, lists, code)
- [ ] Checkbox toggle updates entity
- [ ] Scope navigation works

**Done when:** All entity detail tests pass for both v1 and v2

---

### Phase 6: Search

**Tasks:**

1. `SearchView.vue` - query input + results
2. Full-text search via API
3. Entity type filter
4. Result highlighting
5. Keyboard shortcut (`/` to focus)

**Puppeteer tests:**

- [ ] Search returns matching entities
- [ ] Type filter works
- [ ] Results link to entities
- [ ] Keyboard shortcut focuses input

**Done when:** Search parity achieved

---

### Phase 7: Dashboard

**Tasks:**

1. `DashboardView.vue` - card grid
2. Count cards
3. Breakdown charts (bar graphs)
4. Table displays
5. Validation summary card

**Puppeteer tests:**

- [ ] Cards display correct counts
- [ ] Charts render with correct data
- [ ] Card links navigate correctly

**Done when:** Dashboard parity achieved

---

### Phase 8: Custom Views

**Tasks:**

1. `CustomView.vue` - section-based layout
2. `SectionRenderer.vue` - display modes (properties, cards, list, table,
   content)
3. Scope navigation
4. Link existing / Add new buttons
5. Grouping support

**Puppeteer tests:**

- [ ] Sections render per config
- [ ] Each display mode works
- [ ] Add/link buttons create relations
- [ ] Grouping displays correctly

**Done when:** Custom view parity achieved

---

### Phase 9: Kanban Board

**Tasks:**

1. `KanbanBoard.vue` - column layout
2. `KanbanColumn.vue` - cards container
3. `KanbanCard.vue` - draggable card
4. Drag-drop between columns (vue-draggable)
5. Swimlanes (optional)
6. Property update on drop

**Puppeteer tests:**

- [ ] Columns render per config
- [ ] Cards in correct columns
- [ ] Drag card updates property
- [ ] Swimlanes group correctly

**Done when:** Kanban parity achieved

---

### Phase 10: Markdown Editor + Diagrams

**Tasks:**

1. `MarkdownEditor.vue` - EasyMDE or similar
2. Fullscreen editing mode
3. `MermaidDiagram.vue` - render mermaid blocks
4. Live preview

**Puppeteer tests:**

- [ ] Editor loads with content
- [ ] Toolbar actions work
- [ ] Mermaid diagrams render
- [ ] Fullscreen toggle works

**Done when:** Editor parity achieved

---

### Phase 11: Graph Visualization

**Tasks:**

1. `GraphView.vue` - Cytoscape integration
2. Node/edge rendering from graph data
3. Layout algorithms
4. Click node to navigate
5. Filter by entity type

**Puppeteer tests:**

- [ ] Graph renders nodes and edges
- [ ] Click node navigates
- [ ] Filter reduces visible nodes

**Done when:** Graph parity achieved

---

### Phase 12: Commands + SSE

**Tasks:**

1. Command execution via API
2. SSE stream parsing
3. Toast notifications with progress
4. File/entity/open events
5. Grouped output display

**Puppeteer tests:**

- [ ] Command starts and shows toast
- [ ] Output streams in real-time
- [ ] Completion triggers configured action
- [ ] Cancel stops execution

**Done when:** Command parity achieved

---

### Phase 13: Git Sync

**Tasks:**

1. Git status display (branch, changes)
2. Sync button triggers push/pull
3. Conflict detection
4. Conflict file listing

**Puppeteer tests:**

- [ ] Status shows correct state
- [ ] Sync completes successfully
- [ ] Conflicts display when present

**Done when:** Git sync parity achieved

---

### Phase 14: Settings

**Tasks:**

1. `SettingsView.vue` - user defaults form
2. Property defaults by entity type
3. Relation defaults
4. Persist to `.rela/user-defaults.yaml`

**Puppeteer tests:**

- [ ] Settings form loads current values
- [ ] Save persists changes
- [ ] Defaults apply to new entities

**Done when:** Settings parity achieved

---

### Phase 15: Keyboard Shortcuts + Polish

**Tasks:**

1. `useKeyboard.ts` composable
2. All v1 shortcuts: `/`, `N`, `E`, `H/L`, arrows, `Esc`
3. Hot reload via SSE `/api/events`
4. Toast notifications
5. Responsive design
6. Loading states
7. Error boundaries

**Puppeteer tests:**

- [ ] Each keyboard shortcut works
- [ ] Hot reload updates UI
- [ ] Toasts appear and dismiss
- [ ] Mobile viewport works

**Done when:** All polish tests pass, full parity achieved

---

### Phase 16: Parity Verification

**Tasks:**

1. Run full Puppeteer suite against both v1 and v2
2. Generate parity report
3. Fix any remaining discrepancies
4. Manual QA walkthrough
5. Performance comparison (Lighthouse)

**Done when:**

- 100% parity in Puppeteer tests
- No regressions in functionality
- Performance equal or better

---

### Phase 17: Cutover

**Tasks:**

1. Add redirect: `/` → `/v2/`
2. Update documentation
3. Announce deprecation of v1 routes
4. Monitor for issues

**Done when:** Users accessing v2 by default

---

### Phase 18: Cleanup

**Tasks:**

1. Remove HTMX templates (`internal/dataentry/templates/`)
2. Remove legacy JavaScript (`internal/dataentry/static/app.js`)
3. Remove v1 HTML handlers from `handlers.go`
4. Remove `/api/` endpoints (replaced by `/api/v1/`)
5. Rename `/v2/` to `/` (remove prefix)
6. Rename `/api/v1/` to `/api/`
7. Update all internal links
8. Remove version switcher
9. Clean up unused Go code
10. Update tests

**Files to remove:**

```text
internal/dataentry/templates/*.html     # All 13 template files
internal/dataentry/static/app.js        # 2,058 lines
internal/dataentry/static/htmx.min.js   # HTMX library
internal/dataentry/handlers.go          # HTML handlers (2,590 lines)
```

**Code to remove from:**

```text
internal/dataentry/router.go            # v1 route registrations
internal/dataentry/helpers.go           # Template helper functions
internal/dataentry/templates.go         # Template embedding
```

**Done when:**

- Only Vue app remains
- No HTMX dependencies
- Clean codebase
- All tests pass

---

## Build Integration

### Justfile Updates

```makefile
# Build Vue frontend
build-frontend:
    cd frontend && npm ci && npm run build
    rm -rf internal/dataentry/static/dist
    cp -r frontend/dist internal/dataentry/static/dist

# Dev mode with hot reload
dev-frontend:
    cd frontend && npm run dev

# Full build
build: build-frontend build-cli build-server

# CI
ci: lint test coverage-check build-frontend build
```

### Go Static Serving

```go
// Serve Vue SPA with history mode fallback
func (a *App) registerRoutes(r chi.Router) {
    // API routes
    r.Route("/api/v1", a.registerAPIRoutes)

    // Static files
    r.Handle("/assets/*", http.FileServer(http.FS(staticFS)))

    // SPA fallback - serve index.html for all other routes
    r.NotFound(func(w http.ResponseWriter, r *http.Request) {
        // Don't catch API 404s
        if strings.HasPrefix(r.URL.Path, "/api/") {
            http.NotFound(w, r)
            return
        }

        index, _ := staticFS.ReadFile("dist/index.html")
        w.Header().Set("Content-Type", "text/html")
        w.Write(index)
    })
}
```

---

## Type Generation

Generate TypeScript types from Go structs or OpenAPI:

### Option 1: go-typescript

```bash
# Generate TS types from Go
go-typescript -p internal/model -o frontend/src/types/generated.ts
```

### Option 2: OpenAPI

```yaml
# openapi.yaml (generated or hand-written)
components:
  schemas:
    Entity:
      type: object
      properties:
        id: { type: string }
        type: { type: string }
        properties: { type: object }
        content: { type: string }
        relations:
          type: object
          additionalProperties:
            type: array
            items: { type: string }
```

```bash
# Generate client from OpenAPI
npx openapi-typescript-codegen --input openapi.yaml --output frontend/src/api/generated
```

---

## Benefits of This Approach

1. **Standard REST** - Familiar URL patterns (`/tickets/TKT-001`)
2. **Metamodel-driven** - API adapts to schema changes automatically
3. **Type-safe** - Validation from metamodel on both client and server
4. **Cacheable** - Standard HTTP caching for GET requests
5. **Tooling** - Works with standard REST clients, OpenAPI, etc.
6. **Incremental** - Can migrate piece by piece
7. **Testable** - API easily testable independent of UI

---

## Open Questions

1. **Relation URLs** - Nested (`/tickets/TKT-001/relations`) vs flat
   (`/relations?from=TKT-001`)?
2. **Bulk operations** - Support `PATCH /tickets` for batch updates?
3. **WebSocket** - Use for live updates instead of SSE?
4. **Offline support** - Cache entities in IndexedDB for offline editing?
5. **OpenAPI generation** - Generate from Go code or hand-write spec?
6. **Rate limiting** - Add `X-RateLimit-*` headers for API protection?
