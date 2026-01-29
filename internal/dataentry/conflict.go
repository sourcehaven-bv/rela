package dataentry

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
)

// ConflictSet holds all conflicts being resolved in the current session.
// In Phase 1, conflicts are loaded from synthetic test data.
// In later phases, they will be populated from git merge-tree output.
type ConflictSet struct {
	Files []*ConflictFile
}

// ConflictFile represents a three-way conflict on a single entity/relation file.
type ConflictFile struct {
	ID         string // Unique conflict identifier (e.g., entity ID or filename)
	FilePath   string // Relative path to the file
	EntityID   string // Entity ID (if entity file)
	EntityType string // Entity type (if entity file)
	Title      string // Display title for the conflict

	Base   *ParsedVersion // Common ancestor (nil for new files)
	Ours   *ParsedVersion // Local version
	Theirs *ParsedVersion // Remote version

	Fields       []FieldConflict // Per-field conflicts
	BodyConflict *BodyConflict   // nil if bodies are identical or both unchanged
	Resolved     bool            // Whether this conflict has been resolved
}

// ParsedVersion represents one version of an entity (base, ours, or theirs).
type ParsedVersion struct {
	Properties map[string]string
	Body       string
}

// FieldConflict represents a conflict on a single frontmatter property.
type FieldConflict struct {
	Property   string // Property name
	Label      string // Display label
	BaseValue  string // Value in common ancestor ("" if new)
	OurValue   string // Local value
	TheirValue string // Remote value
	Status     string // "conflict", "auto-ours", "auto-theirs", "unchanged"
}

// BodyConflict represents a conflict on the markdown body content.
type BodyConflict struct {
	BaseBody     string
	OurBody      string
	TheirBody    string
	Hunks        []DiffHunk // Three-way diff hunks
	CanAutoMerge bool       // True if changes don't overlap
}

// DiffHunk represents a section of the body that differs between versions.
type DiffHunk struct {
	Index  int // Stable index for form references (0-based)
	Lines  []DiffLine
	Source string // "context", "ours", "theirs", "both", "conflict"
}

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type    string // "context", "add-ours", "add-theirs", "del-ours", "del-theirs"
	Content string
	LineNo  int // Original line number (0-based, -1 if not applicable)
}

// ComputeFieldConflicts performs a three-way comparison of frontmatter properties.
// It classifies each field as unchanged, auto-resolved (only one side changed),
// or conflicting (both sides changed differently).
func ComputeFieldConflicts(base, ours, theirs *ParsedVersion, fields []string) []FieldConflict {
	result := make([]FieldConflict, 0, len(fields))

	baseProps := make(map[string]string)
	if base != nil {
		baseProps = base.Properties
	}

	for _, prop := range fields {
		baseVal := baseProps[prop]
		ourVal := ours.Properties[prop]
		theirVal := theirs.Properties[prop]

		fc := FieldConflict{
			Property:   prop,
			BaseValue:  baseVal,
			OurValue:   ourVal,
			TheirValue: theirVal,
		}

		ourChanged := ourVal != baseVal
		theirChanged := theirVal != baseVal

		switch {
		case !ourChanged && !theirChanged:
			fc.Status = "unchanged"
		case ourChanged && !theirChanged:
			fc.Status = "auto-ours"
		case !ourChanged && theirChanged:
			fc.Status = "auto-theirs"
		case ourVal == theirVal:
			// Both changed to the same value — no conflict.
			fc.Status = "auto-ours"
		default:
			fc.Status = "conflict"
		}

		result = append(result, fc)
	}

	return result
}

// ComputeBodyConflict performs a three-way diff on body text.
// Returns nil if no conflict exists (both sides identical or only one changed).
func ComputeBodyConflict(base, ours, theirs *ParsedVersion) *BodyConflict {
	baseBody := ""
	if base != nil {
		baseBody = base.Body
	}
	ourBody := ours.Body
	theirBody := theirs.Body

	ourChanged := ourBody != baseBody
	theirChanged := theirBody != baseBody

	// No conflict cases
	if !ourChanged && !theirChanged {
		return nil
	}
	if ourChanged && !theirChanged {
		return nil // Take ours automatically
	}
	if !ourChanged && theirChanged {
		return nil // Take theirs automatically
	}
	if ourBody == theirBody {
		return nil // Both changed identically
	}

	// Both sides changed differently — compute three-way diff
	hunks := threeWayDiff(baseBody, ourBody, theirBody)
	canAutoMerge := !hasOverlappingChanges(hunks)

	return &BodyConflict{
		BaseBody:     baseBody,
		OurBody:      ourBody,
		TheirBody:    theirBody,
		Hunks:        hunks,
		CanAutoMerge: canAutoMerge,
	}
}

// threeWayDiff computes a three-way diff between base, ours, and theirs.
// It uses LCS-based two-way diffs and merges them into a unified view.
func threeWayDiff(base, ours, theirs string) []DiffHunk {
	baseLines := splitLines(base)
	ourLines := splitLines(ours)
	theirLines := splitLines(theirs)

	// Compute two-way diffs against base
	ourOps := diffLines(baseLines, ourLines)
	theirOps := diffLines(baseLines, theirLines)

	// Merge into three-way hunks
	return mergeOps(baseLines, ourOps, theirOps)
}

// diffOp represents a single diff operation.
type diffOp struct {
	Type    string // "equal", "delete", "insert"
	BaseLn  int    // Line number in base (-1 for inserts)
	NewLn   int    // Line number in new (-1 for deletes)
	Content string
}

// diffLines computes the diff between two sets of lines using LCS.
func diffLines(a, b []string) []diffOp {
	m, n := len(a), len(b)

	// Build LCS table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			switch {
			case a[i-1] == b[j-1]:
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] >= dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			default:
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to build ops
	var ops []diffOp
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			ops = append(ops, diffOp{Type: "equal", BaseLn: i - 1, NewLn: j - 1, Content: a[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			ops = append(ops, diffOp{Type: "insert", BaseLn: -1, NewLn: j - 1, Content: b[j-1]})
			j--
		default:
			ops = append(ops, diffOp{Type: "delete", BaseLn: i - 1, NewLn: -1, Content: a[i-1]})
			i--
		}
	}

	// Reverse (backtracking produces reverse order)
	for left, right := 0, len(ops)-1; left < right; left, right = left+1, right-1 {
		ops[left], ops[right] = ops[right], ops[left]
	}

	return ops
}

// mergeOps merges two sets of diff operations (ours vs base, theirs vs base)
// into a unified three-way diff with hunks.
func mergeOps(baseLines []string, ourOps, theirOps []diffOp) []DiffHunk {
	// Track changes per base line position.
	// Position i means "between base line i-1 and i" for inserts,
	// or "base line i" for deletes.
	ourChanges := buildChangeMap(ourOps)
	theirChanges := buildChangeMap(theirOps)

	var hunks []DiffHunk
	var currentLines []DiffLine
	currentSource := ""
	idx := 0

	appendHunk := func(h DiffHunk) {
		h.Index = idx
		idx++
		hunks = append(hunks, h)
	}

	flushHunk := func() {
		if len(currentLines) > 0 {
			appendHunk(DiffHunk{Lines: currentLines, Source: currentSource})
			currentLines = nil
			currentSource = ""
		}
	}

	maxPos := len(baseLines) + 1
	for pos := 0; pos <= maxPos; pos++ {
		ourC := ourChanges[pos]
		theirC := theirChanges[pos]

		// Handle inserts before this base line
		ourIns := ourC.inserts
		theirIns := theirC.inserts

		switch {
		case len(ourIns) > 0 && len(theirIns) > 0:
			// Both sides inserted at same position — conflict
			flushHunk()
			var lines []DiffLine
			for _, l := range ourIns {
				lines = append(lines, DiffLine{Type: "add-ours", Content: l, LineNo: -1})
			}
			for _, l := range theirIns {
				lines = append(lines, DiffLine{Type: "add-theirs", Content: l, LineNo: -1})
			}
			appendHunk(DiffHunk{Lines: lines, Source: "conflict"})
		case len(ourIns) > 0:
			flushHunk()
			var lines []DiffLine
			for _, l := range ourIns {
				lines = append(lines, DiffLine{Type: "add-ours", Content: l, LineNo: -1})
			}
			appendHunk(DiffHunk{Lines: lines, Source: "ours"})
		case len(theirIns) > 0:
			flushHunk()
			var lines []DiffLine
			for _, l := range theirIns {
				lines = append(lines, DiffLine{Type: "add-theirs", Content: l, LineNo: -1})
			}
			appendHunk(DiffHunk{Lines: lines, Source: "theirs"})
		}

		if pos >= len(baseLines) {
			break
		}

		// Handle this base line (equal, deleted by ours, deleted by theirs, or both)
		ourDel := ourC.deleted
		theirDel := theirC.deleted

		switch {
		case !ourDel && !theirDel:
			// Context line
			if currentSource != "context" {
				flushHunk()
				currentSource = "context"
			}
			currentLines = append(currentLines, DiffLine{
				Type: "context", Content: baseLines[pos], LineNo: pos,
			})
		case ourDel && theirDel:
			// Both deleted — agreed, show as context-delete
			flushHunk()
			appendHunk(DiffHunk{
				Lines:  []DiffLine{{Type: "del-ours", Content: baseLines[pos], LineNo: pos}},
				Source: "both",
			})
		case ourDel:
			flushHunk()
			appendHunk(DiffHunk{
				Lines:  []DiffLine{{Type: "del-ours", Content: baseLines[pos], LineNo: pos}},
				Source: "ours",
			})
		case theirDel:
			flushHunk()
			appendHunk(DiffHunk{
				Lines:  []DiffLine{{Type: "del-theirs", Content: baseLines[pos], LineNo: pos}},
				Source: "theirs",
			})
		}
	}

	flushHunk()
	return hunks
}

type changeInfo struct {
	deleted bool
	inserts []string
}

// buildChangeMap processes diff ops into a position-indexed change map.
// Position corresponds to base line index. Inserts at position i mean
// "inserted before base line i".
func buildChangeMap(ops []diffOp) map[int]changeInfo {
	changes := make(map[int]changeInfo)
	basePos := 0
	for _, op := range ops {
		switch op.Type {
		case "equal":
			basePos++
		case "delete":
			c := changes[basePos]
			c.deleted = true
			changes[basePos] = c
			basePos++
		case "insert":
			c := changes[basePos]
			c.inserts = append(c.inserts, op.Content)
			changes[basePos] = c
		}
	}
	return changes
}

// hasOverlappingChanges returns true if any hunk is a "conflict" source,
// meaning both sides made changes at the same location.
func hasOverlappingChanges(hunks []DiffHunk) bool {
	for _, h := range hunks {
		if h.Source == "conflict" {
			return true
		}
	}
	return false
}

// splitLines splits text into lines, handling empty input.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// HasConflicts returns true if there are any unresolved field or body conflicts.
func (cf *ConflictFile) HasConflicts() bool {
	for _, f := range cf.Fields {
		if f.Status == "conflict" {
			return true
		}
	}
	return cf.BodyConflict != nil && !cf.BodyConflict.CanAutoMerge
}

// CountConflictingFields returns the number of fields with status "conflict".
func (cf *ConflictFile) CountConflictingFields() int {
	count := 0
	for _, f := range cf.Fields {
		if f.Status == "conflict" {
			count++
		}
	}
	return count
}

// ConflictCount returns the number of unresolved conflict files.
func (cs *ConflictSet) ConflictCount() int {
	count := 0
	for _, f := range cs.Files {
		if !f.Resolved {
			count++
		}
	}
	return count
}

// GetConflict returns a conflict file by ID, or nil if not found.
func (cs *ConflictSet) GetConflict(id string) *ConflictFile {
	for _, f := range cs.Files {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// BuildTestConflictSet creates a synthetic conflict set for testing the UI.
// This simulates what git merge-tree would produce in Phase 4.
func BuildTestConflictSet() *ConflictSet {
	// Conflict 1: A ticket with field-level and body conflicts
	ticket := &ConflictFile{
		ID:         "TKT-004",
		FilePath:   "entities/ticket/TKT-004.md",
		EntityID:   "TKT-004",
		EntityType: "ticket",
		Title:      "Rate limiting for API endpoints",
		Base: &ParsedVersion{
			Properties: map[string]string{
				"title":           "Rate limiting for API endpoints",
				"status":          "open",
				"priority":        "medium",
				"assignee":        "alice",
				"reporter":        "bob",
				"estimated_hours": "8",
				"due_date":        "2025-03-15",
			},
			Body: "We need to implement rate limiting for all public API endpoints.\n\nRequirements:\n- 100 requests per minute per API key\n- Return 429 status when exceeded\n- Include retry-after header",
		},
		Ours: &ParsedVersion{
			Properties: map[string]string{
				"title":           "Rate limiting for API endpoints",
				"status":          "in-progress",
				"priority":        "high",
				"assignee":        "alice",
				"reporter":        "bob",
				"estimated_hours": "12",
				"due_date":        "2025-03-15",
			},
			Body: "We need to implement rate limiting for all public API endpoints.\n\nRequirements:\n- 100 requests per minute per API key\n- Return 429 status when exceeded\n- Include retry-after header\n- Log rate limit violations\n\nImplementation notes:\n- Use token bucket algorithm\n- Store counters in Redis",
		},
		Theirs: &ParsedVersion{
			Properties: map[string]string{
				"title":           "Rate limiting for API endpoints",
				"status":          "resolved",
				"priority":        "critical",
				"assignee":        "charlie",
				"reporter":        "bob",
				"estimated_hours": "8",
				"due_date":        "2025-04-01",
			},
			Body: "We need to implement rate limiting for all public API endpoints.\n\nRequirements:\n- 200 requests per minute per API key\n- Return 429 status when exceeded\n- Include retry-after header\n- Whitelist internal services\n\nAcceptance criteria:\n- Load test passes with 1000 concurrent users",
		},
	}

	fields := []string{"title", "status", "priority", "assignee", "reporter", "estimated_hours", "due_date"}
	ticket.Fields = ComputeFieldConflicts(ticket.Base, ticket.Ours, ticket.Theirs, fields)
	ticket.BodyConflict = ComputeBodyConflict(ticket.Base, ticket.Ours, ticket.Theirs)

	// Add labels from field definitions
	labelMap := map[string]string{
		"title": "Title", "status": "Status", "priority": "Priority",
		"assignee": "Assignee", "reporter": "Reporter",
		"estimated_hours": "Est. Hours", "due_date": "Due Date",
	}
	for i := range ticket.Fields {
		if l, ok := labelMap[ticket.Fields[i].Property]; ok {
			ticket.Fields[i].Label = l
		}
	}

	// Conflict 2: A simpler conflict — category description changed on both sides
	category := &ConflictFile{
		ID:         "backend",
		FilePath:   "entities/category/backend.md",
		EntityID:   "backend",
		EntityType: "category",
		Title:      "Backend",
		Base: &ParsedVersion{
			Properties: map[string]string{
				"name":        "Backend",
				"description": "Backend services and APIs",
				"color":       "#3b82f6",
			},
			Body: "",
		},
		Ours: &ParsedVersion{
			Properties: map[string]string{
				"name":        "Backend",
				"description": "Backend services, APIs, and infrastructure",
				"color":       "#3b82f6",
			},
			Body: "",
		},
		Theirs: &ParsedVersion{
			Properties: map[string]string{
				"name":        "Backend",
				"description": "Server-side services and REST APIs",
				"color":       "#2563eb",
			},
			Body: "",
		},
	}

	catFields := []string{"name", "description", "color"}
	category.Fields = ComputeFieldConflicts(category.Base, category.Ours, category.Theirs, catFields)
	category.BodyConflict = ComputeBodyConflict(category.Base, category.Ours, category.Theirs)
	catLabelMap := map[string]string{"name": "Name", "description": "Description", "color": "Color"}
	for i := range category.Fields {
		if l, ok := catLabelMap[category.Fields[i].Property]; ok {
			category.Fields[i].Label = l
		}
	}

	// Conflict 3: A new entity that both sides created differently
	newTicket := &ConflictFile{
		ID:         "TKT-010",
		FilePath:   "entities/ticket/TKT-010.md",
		EntityID:   "TKT-010",
		EntityType: "ticket",
		Title:      "New ticket created on both sides",
		Base:       nil, // New file — no common ancestor
		Ours: &ParsedVersion{
			Properties: map[string]string{
				"title":    "Add caching layer",
				"status":   "open",
				"priority": "high",
				"assignee": "alice",
			},
			Body: "We should add a caching layer to improve response times.",
		},
		Theirs: &ParsedVersion{
			Properties: map[string]string{
				"title":    "Implement response caching",
				"status":   "open",
				"priority": "medium",
				"assignee": "dave",
			},
			Body: "Adding HTTP response caching using ETags and Cache-Control headers.",
		},
	}

	newFields := []string{"title", "status", "priority", "assignee"}
	newTicket.Fields = ComputeFieldConflicts(newTicket.Base, newTicket.Ours, newTicket.Theirs, newFields)
	newTicket.BodyConflict = ComputeBodyConflict(newTicket.Base, newTicket.Ours, newTicket.Theirs)
	for i := range newTicket.Fields {
		if l, ok := labelMap[newTicket.Fields[i].Property]; ok {
			newTicket.Fields[i].Label = l
		}
	}

	// Conflict 4: A design document with multiple body conflicts at different locations
	design := &ConflictFile{
		ID:         "DES-003",
		FilePath:   "entities/decision/DES-003.md",
		EntityID:   "DES-003",
		EntityType: "decision",
		Title:      "Authentication architecture",
		Base: &ParsedVersion{
			Properties: map[string]string{
				"title":  "Authentication architecture",
				"status": "proposed",
				"author": "alice",
			},
			Body: "## Context\n\nWe need a unified authentication system for all services.\nCurrently each service handles auth independently.\n\n## Decision\n\nWe will use JWT tokens for service-to-service auth.\nTokens will be issued by a central auth service.\nToken lifetime will be 1 hour.\n\n## Consequences\n\n- All services must validate JWT signatures\n- Auth service becomes a critical dependency\n- Need to handle token refresh gracefully\n- Must implement token revocation list",
		},
		Ours: &ParsedVersion{
			Properties: map[string]string{
				"title":  "Authentication architecture",
				"status": "accepted",
				"author": "alice",
			},
			Body: "## Context\n\nWe need a unified authentication system for all services.\nCurrently each service handles auth independently.\nThis has led to inconsistent security policies.\n\n## Decision\n\nWe will use JWT tokens for service-to-service auth.\nTokens will be issued by a central auth service.\nToken lifetime will be 30 minutes.\nRefresh tokens will have a 7-day lifetime.\n\n## Consequences\n\n- All services must validate JWT signatures\n- Auth service becomes a critical dependency\n- Need to handle token refresh gracefully\n- Must implement token revocation list\n- Add monitoring for auth service uptime",
		},
		Theirs: &ParsedVersion{
			Properties: map[string]string{
				"title":  "Authentication architecture",
				"status": "accepted",
				"author": "alice",
			},
			Body: "## Context\n\nWe need a unified authentication system for all services.\nCurrently each service handles auth independently.\nSecurity audit flagged this as a high-risk issue.\n\n## Decision\n\nWe will use JWT tokens for service-to-service auth.\nTokens will be issued by a central auth service.\nToken lifetime will be 15 minutes.\nAll tokens must include service identity claims.\n\n## Consequences\n\n- All services must validate JWT signatures\n- Auth service becomes a critical dependency\n- Need to handle token refresh gracefully\n- Must implement token revocation list\n- Implement circuit breaker for auth service calls",
		},
	}

	designFields := []string{"title", "status", "author"}
	design.Fields = ComputeFieldConflicts(design.Base, design.Ours, design.Theirs, designFields)
	design.BodyConflict = ComputeBodyConflict(design.Base, design.Ours, design.Theirs)
	designLabelMap := map[string]string{"title": "Title", "status": "Status", "author": "Author"}
	for i := range design.Fields {
		if l, ok := designLabelMap[design.Fields[i].Property]; ok {
			design.Fields[i].Label = l
		}
	}

	return &ConflictSet{
		Files: []*ConflictFile{ticket, category, newTicket, design},
	}
}

// ApplyResolution applies user choices to a conflict file.
// fieldChoices maps property name to "ours" or "theirs".
// hunkChoices maps hunk index to "ours" or "theirs" for conflict hunks.
func (cf *ConflictFile) ApplyResolution(fieldChoices map[string]string, hunkChoices map[int]string) (resolvedProps map[string]string, resolvedBody string, err error) {
	resolvedProps = make(map[string]string)

	// Resolve each field
	for _, f := range cf.Fields {
		switch f.Status {
		case "unchanged":
			resolvedProps[f.Property] = f.OurValue // Same everywhere
		case "auto-ours":
			resolvedProps[f.Property] = f.OurValue
		case "auto-theirs":
			resolvedProps[f.Property] = f.TheirValue
		case "conflict":
			choice, ok := fieldChoices[f.Property]
			if !ok {
				return nil, "", fmt.Errorf("no resolution for conflicting field %q", f.Property)
			}
			switch choice {
			case "ours":
				resolvedProps[f.Property] = f.OurValue
			case "theirs":
				resolvedProps[f.Property] = f.TheirValue
			default:
				return nil, "", fmt.Errorf("invalid choice %q for field %q", choice, f.Property)
			}
		}
	}

	// Resolve body
	if cf.BodyConflict == nil {
		// No body conflict — take whichever side changed (or base if neither)
		if cf.Ours != nil {
			resolvedBody = cf.Ours.Body
		}
	} else {
		resolvedBody, err = resolveBodyFromHunks(cf.BodyConflict, hunkChoices)
		if err != nil {
			return nil, "", err
		}
	}

	cf.Resolved = true
	return resolvedProps, resolvedBody, nil
}

// resolveBodyFromHunks assembles the resolved body by iterating hunks.
// Non-conflict hunks auto-resolve; conflict hunks use the per-hunk choice.
func resolveBodyFromHunks(bc *BodyConflict, hunkChoices map[int]string) (string, error) {
	var lines []string
	for _, hunk := range bc.Hunks {
		if hunk.Source == "conflict" {
			choice, ok := hunkChoices[hunk.Index]
			if !ok {
				return "", fmt.Errorf("no resolution for body conflict at hunk %d", hunk.Index)
			}
			for _, line := range hunk.Lines {
				switch {
				case choice == "ours" && line.Type == "add-ours":
					lines = append(lines, line.Content)
				case choice == "theirs" && line.Type == "add-theirs":
					lines = append(lines, line.Content)
				}
			}
			continue
		}
		// Non-conflict hunks: include additions, skip deletions
		for _, line := range hunk.Lines {
			switch line.Type {
			case "context":
				lines = append(lines, line.Content)
			case "add-ours", "add-theirs":
				lines = append(lines, line.Content)
			case "del-ours", "del-theirs":
				// Line was deleted — skip it
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

// CountConflictHunks returns the number of hunks with source "conflict".
func (bc *BodyConflict) CountConflictHunks() int {
	count := 0
	for _, h := range bc.Hunks {
		if h.Source == "conflict" {
			count++
		}
	}
	return count
}

// BuildConflictSetFromGit creates a ConflictSet by examining the diff between
// HEAD (ours) and the upstream branch (theirs), using the merge-base as the
// common ancestor. Only markdown files (.md) are considered.
func BuildConflictSetFromGit(repoRoot, branch string) (*ConflictSet, error) {
	upstream := "origin/" + branch

	// 1. Find the merge base
	mergeBase, err := gitOutput(repoRoot, "merge-base", "HEAD", upstream)
	if err != nil {
		return nil, fmt.Errorf("finding merge-base: %w", err)
	}
	mergeBase = strings.TrimSpace(mergeBase)
	if mergeBase == "" {
		return nil, fmt.Errorf("no merge-base between HEAD and %s", upstream)
	}

	// 2. Find files that differ between ours vs base AND theirs vs base
	//    (files changed on both sides are potential conflicts)
	oursChanged, err := gitOutput(repoRoot, "diff", "--name-only", mergeBase, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("diff ours: %w", err)
	}
	theirsChanged, err := gitOutput(repoRoot, "diff", "--name-only", mergeBase, upstream)
	if err != nil {
		return nil, fmt.Errorf("diff theirs: %w", err)
	}

	oursFiles := parseFileList(oursChanged)
	theirsFiles := parseFileList(theirsChanged)

	// Find intersection (files changed on both sides)
	theirsSet := make(map[string]bool, len(theirsFiles))
	for _, f := range theirsFiles {
		theirsSet[f] = true
	}

	var conflictPaths []string
	for _, f := range oursFiles {
		if theirsSet[f] && strings.HasSuffix(strings.ToLower(f), ".md") {
			conflictPaths = append(conflictPaths, f)
		}
	}

	if len(conflictPaths) == 0 {
		return &ConflictSet{}, nil
	}

	// 3. For each conflicting file, extract the three versions
	var files []*ConflictFile
	for _, path := range conflictPaths {
		cf, err := buildConflictFile(repoRoot, mergeBase, upstream, path)
		if err != nil {
			// Skip files we can't parse (binary files, etc.)
			continue
		}
		// Only include if there are actual conflicts
		if cf.HasConflicts() {
			files = append(files, cf)
		}
	}

	return &ConflictSet{Files: files}, nil
}

// buildConflictFile extracts three versions of a file and builds a ConflictFile.
func buildConflictFile(repoRoot, mergeBase, upstream, path string) (*ConflictFile, error) {
	// Extract three versions
	baseContent, baseErr := gitOutput(repoRoot, "show", mergeBase+":"+path)
	oursContent, oursErr := gitOutput(repoRoot, "show", "HEAD:"+path)
	theirsContent, theirsErr := gitOutput(repoRoot, "show", upstream+":"+path)

	// At least ours and theirs must exist
	if oursErr != nil && theirsErr != nil {
		return nil, fmt.Errorf("cannot read ours or theirs for %s", path)
	}

	// Parse each version
	var baseParsed, oursParsed, theirsParsed *ParsedVersion

	if baseErr == nil {
		baseParsed = parseVersionFromContent(baseContent)
	}
	if oursErr == nil {
		oursParsed = parseVersionFromContent(oursContent)
	} else {
		oursParsed = &ParsedVersion{Properties: map[string]string{}}
	}
	if theirsErr == nil {
		theirsParsed = parseVersionFromContent(theirsContent)
	} else {
		theirsParsed = &ParsedVersion{Properties: map[string]string{}}
	}

	// Derive entity info from path
	entityType, entityID := entityInfoFromPath(path)
	title := entityID
	if t, ok := oursParsed.Properties["title"]; ok && t != "" {
		title = t
	}

	// Collect all property names across versions
	allProps := collectPropertyNames(baseParsed, oursParsed, theirsParsed)

	cf := &ConflictFile{
		ID:         entityID,
		FilePath:   path,
		EntityID:   entityID,
		EntityType: entityType,
		Title:      title,
		Base:       baseParsed,
		Ours:       oursParsed,
		Theirs:     theirsParsed,
	}

	cf.Fields = ComputeFieldConflicts(baseParsed, oursParsed, theirsParsed, allProps)
	cf.BodyConflict = ComputeBodyConflict(baseParsed, oursParsed, theirsParsed)

	// Add labels from property names (capitalize first letter)
	for i := range cf.Fields {
		cf.Fields[i].Label = capitalizeFirst(cf.Fields[i].Property)
	}

	return cf, nil
}

// parseVersionFromContent parses markdown content into a ParsedVersion.
func parseVersionFromContent(content string) *ParsedVersion {
	doc, err := markdown.ParseDocument(content)
	if err != nil {
		return &ParsedVersion{Properties: map[string]string{}, Body: content}
	}

	props := make(map[string]string)
	for k, v := range doc.Frontmatter {
		props[k] = fmt.Sprintf("%v", v)
	}

	return &ParsedVersion{
		Properties: props,
		Body:       doc.Content,
	}
}

// entityInfoFromPath extracts entity type and ID from a file path like "entities/ticket/TKT-001.md".
func entityInfoFromPath(path string) (entityType, entityID string) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Strip .md extension for ID
	entityID = strings.TrimSuffix(base, filepath.Ext(base))

	// Entity type is the parent directory name
	entityType = filepath.Base(dir)

	return entityType, entityID
}

// collectPropertyNames gathers all unique property names from three versions.
func collectPropertyNames(base, ours, theirs *ParsedVersion) []string {
	seen := make(map[string]bool)
	var names []string

	addProps := func(pv *ParsedVersion) {
		if pv == nil {
			return
		}
		for k := range pv.Properties {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		}
	}

	addProps(ours)   // Ours first so our ordering takes priority
	addProps(theirs) // Then theirs
	addProps(base)   // Then base

	return names
}

// capitalizeFirst capitalizes the first letter of a string and replaces underscores with spaces.
func capitalizeFirst(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseFileList splits git output into a list of file paths.
func parseFileList(output string) []string {
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if f := strings.TrimSpace(line); f != "" {
			files = append(files, f)
		}
	}
	return files
}

// gitOutput runs a git command and returns its stdout.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

// FormatResolvedDocument creates a markdown document from resolved properties and body.
func FormatResolvedDocument(props map[string]string, body string) (string, error) {
	fm := make(map[string]interface{}, len(props))
	for k, v := range props {
		fm[k] = v
	}
	return markdown.FormatDocument(fm, body)
}
