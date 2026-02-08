package git

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	// relationMatchCount is the expected number of matches from relationFileRegex (full match + 3 groups).
	relationMatchCount = 4
	// maxCommitMsgLen is the maximum length for a commit message line.
	maxCommitMsgLen = 72
)

// ChangeSet represents analyzed changes for commit message generation.
type ChangeSet struct {
	Added     []EntityChange
	Modified  []EntityChange
	Deleted   []EntityRef
	Relations []RelationChange
}

// EntityChange represents a changed entity.
type EntityChange struct {
	Type         string
	ID           string
	PropsChanged []string
	BodyChanged  bool
	IsNew        bool
}

// EntityRef is a reference to an entity.
type EntityRef struct {
	Type string
	ID   string
}

// RelationChange represents a changed relation.
type RelationChange struct {
	From    string
	RelType string
	To      string
	IsNew   bool
}

// AnalyzeChanges examines staged/unstaged changes and returns a ChangeSet.
func (g *Ops) AnalyzeChanges() (*ChangeSet, error) {
	out, err := runGit(g.root, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	cs := &ChangeSet{}

	for _, line := range strings.Split(out, "\n") {
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		path := strings.TrimSpace(line[3:])

		// Only process entities/ and relations/
		if !strings.HasPrefix(path, "entities/") && !strings.HasPrefix(path, "relations/") {
			continue
		}

		if strings.HasPrefix(path, "entities/") {
			g.processEntityChange(cs, status, path)
		} else if strings.HasPrefix(path, "relations/") {
			g.processRelationChange(cs, status, path)
		}
	}

	return cs, nil
}

func (g *Ops) processEntityChange(cs *ChangeSet, status, path string) {
	// Parse path: entities/<type>/<id>.md
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return
	}

	entityType := parts[1]
	id := strings.TrimSuffix(parts[2], ".md")

	switch {
	case strings.Contains(status, "A") || strings.Contains(status, "?"):
		cs.Added = append(cs.Added, EntityChange{
			Type:  entityType,
			ID:    id,
			IsNew: true,
		})
	case strings.Contains(status, "D"):
		cs.Deleted = append(cs.Deleted, EntityRef{
			Type: entityType,
			ID:   id,
		})
	case strings.Contains(status, "M") || strings.Contains(status, " "):
		change := EntityChange{
			Type: entityType,
			ID:   id,
		}
		// Try to get what changed
		props, bodyChanged := g.analyzeEntityDiff(path)
		change.PropsChanged = props
		change.BodyChanged = bodyChanged
		cs.Modified = append(cs.Modified, change)
	}
}

var relationFileRegex = regexp.MustCompile(`^(.+)--(.+)--(.+)\.md$`)

func (g *Ops) processRelationChange(cs *ChangeSet, status, path string) {
	// Parse path: relations/<from>--<type>--<to>.md
	filename := filepath.Base(path)
	matches := relationFileRegex.FindStringSubmatch(filename)
	if len(matches) != relationMatchCount {
		return
	}

	rel := RelationChange{
		From:    matches[1],
		RelType: matches[2],
		To:      matches[3],
		IsNew:   strings.Contains(status, "A") || strings.Contains(status, "?"),
	}
	cs.Relations = append(cs.Relations, rel)
}

func (g *Ops) analyzeEntityDiff(path string) (propsChanged []string, bodyChanged bool) {
	// Get diff for the file
	out, err := runGit(g.root, "diff", "HEAD", "--", path)
	if err != nil {
		// Try without HEAD for untracked
		out, err = runGit(g.root, "diff", "--", path)
		if err != nil {
			return nil, false
		}
	}

	inFrontmatter := false
	seenFrontmatterEnd := false

	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-") {
			continue
		}
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}

		content := line[1:]

		if content == "---" {
			if !inFrontmatter {
				inFrontmatter = true
			} else {
				seenFrontmatterEnd = true
				inFrontmatter = false
			}
			continue
		}

		if inFrontmatter {
			// Extract property name
			if idx := strings.Index(content, ":"); idx > 0 {
				prop := strings.TrimSpace(content[:idx])
				if prop != "" && !contains(propsChanged, prop) {
					propsChanged = append(propsChanged, prop)
				}
			}
		} else if seenFrontmatterEnd {
			bodyChanged = true
		}
	}

	return propsChanged, bodyChanged
}

// GenerateCommitMessage creates a human-readable commit message from changes.
//
//nolint:gocognit // commit message generation requires handling multiple cases
func (cs *ChangeSet) GenerateCommitMessage() string {
	var parts []string

	// Group by type for cleaner messages
	if len(cs.Added) == 1 {
		e := cs.Added[0]
		parts = append(parts, fmt.Sprintf("Add %s %s", e.Type, e.ID))
	} else if len(cs.Added) > 1 {
		byType := groupByType(cs.Added)
		for t, entities := range byType {
			if len(entities) <= 3 {
				ids := extractIDs(entities)
				parts = append(parts, fmt.Sprintf("Add %s %s", t, strings.Join(ids, ", ")))
			} else {
				parts = append(parts, fmt.Sprintf("Add %d %ss", len(entities), t))
			}
		}
	}

	if len(cs.Modified) == 1 {
		e := cs.Modified[0]
		switch {
		case len(e.PropsChanged) > 0:
			parts = append(parts, fmt.Sprintf("%s: update %s", e.ID, strings.Join(e.PropsChanged, ", ")))
		case e.BodyChanged:
			parts = append(parts, fmt.Sprintf("%s: update description", e.ID))
		default:
			parts = append(parts, fmt.Sprintf("%s: update", e.ID))
		}
	} else if len(cs.Modified) > 1 {
		byType := groupByType(cs.Modified)
		for t, entities := range byType {
			if len(entities) <= 3 {
				ids := extractIDs(entities)
				parts = append(parts, fmt.Sprintf("Update %s %s", t, strings.Join(ids, ", ")))
			} else {
				parts = append(parts, fmt.Sprintf("Update %d %ss", len(entities), t))
			}
		}
	}

	if len(cs.Deleted) == 1 {
		e := cs.Deleted[0]
		parts = append(parts, fmt.Sprintf("Remove %s %s", e.Type, e.ID))
	} else if len(cs.Deleted) > 1 {
		parts = append(parts, fmt.Sprintf("Remove %d entities", len(cs.Deleted)))
	}

	if len(cs.Relations) > 0 {
		newRels := 0
		for _, r := range cs.Relations {
			if r.IsNew {
				newRels++
			}
		}
		if newRels == 1 {
			for _, r := range cs.Relations {
				if r.IsNew {
					parts = append(parts, fmt.Sprintf("Link %s -> %s", r.From, r.To))
					break
				}
			}
		} else if newRels > 1 {
			parts = append(parts, fmt.Sprintf("Add %d relations", newRels))
		}
	}

	if len(parts) == 0 {
		return "Update entities"
	}

	// Join with semicolons, keep it concise
	msg := strings.Join(parts, "; ")
	if len(msg) > maxCommitMsgLen {
		// Truncate for git convention
		msg = msg[:69] + "..."
	}

	return msg
}

func groupByType(changes []EntityChange) map[string][]EntityChange {
	result := make(map[string][]EntityChange)
	for _, c := range changes {
		result[c.Type] = append(result[c.Type], c)
	}
	return result
}

func extractIDs(changes []EntityChange) []string {
	ids := make([]string, len(changes))
	for i, c := range changes {
		ids[i] = c.ID
	}
	sort.Strings(ids)
	return ids
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
