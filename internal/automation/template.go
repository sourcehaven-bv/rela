package automation

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// TemplateVars holds variables available for template interpolation.
type TemplateVars struct {
	// Now is the current time function (injectable for testing).
	Now func() time.Time

	// User holds user identity information.
	User UserVars
}

// UserVars holds user identity information.
type UserVars struct {
	Name  string
	Email string
}

// DefaultTemplateVars returns template vars with current time and git user info.
func DefaultTemplateVars() TemplateVars {
	return TemplateVars{
		Now:  time.Now,
		User: GetGitUser(),
	}
}

// GetGitUser retrieves user.name and user.email from git config.
func GetGitUser() UserVars {
	vars := UserVars{
		Name:  os.Getenv("USER"),
		Email: "",
	}

	// Try git config user.name
	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		vars.Name = strings.TrimSpace(string(out))
	}

	// Try git config user.email
	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		vars.Email = strings.TrimSpace(string(out))
	}

	return vars
}

// Interpolate replaces template variables in a string.
// Supported variables:
//   - {{now}}         - Current timestamp (ISO 8601)
//   - {{today}}       - Current date (YYYY-MM-DD)
//   - {{user.name}}   - Git user name
//   - {{user.email}}  - Git user email
//   - {{entity.id}}   - Current entity ID
//   - {{entity.type}} - Current entity type
//   - {{old.<prop>}}  - Previous value of a property
//   - {{new.<prop>}}  - New value of a property
func Interpolate(template string, vars TemplateVars, entity, oldEntity *model.Entity) string {
	if !strings.Contains(template, "{{") {
		return template
	}

	now := vars.Now()
	if vars.Now == nil {
		now = time.Now()
	}

	replacements := map[string]string{
		"{{now}}":        now.Format(time.RFC3339),
		"{{today}}":      now.Format("2006-01-02"),
		"{{user.name}}":  vars.User.Name,
		"{{user.email}}": vars.User.Email,
	}

	if entity != nil {
		replacements["{{entity.id}}"] = entity.ID
		replacements["{{entity.type}}"] = entity.Type
	}

	// Process the template
	result := template

	// Simple replacements
	for key, value := range replacements {
		result = strings.ReplaceAll(result, key, value)
	}

	// Handle dynamic property references: {{old.prop}} and {{new.prop}}
	result = interpolatePropertyRefs(result, "old.", oldEntity)
	result = interpolatePropertyRefs(result, "new.", entity)

	return result
}

// InterpolateSafeOnly replaces only safe template variables in a string.
// This is used for Lua code where entity properties should be accessed via globals,
// not interpolated into the code (to prevent injection attacks).
// Supported safe variables:
//   - {{now}}        - Current timestamp (ISO 8601)
//   - {{today}}      - Current date (YYYY-MM-DD)
//   - {{user.name}}  - Git user name
//   - {{user.email}} - Git user email
//
// NOT interpolated (left as-is or accessed via Lua globals):
//   - {{entity.*}}   - Entity fields
//   - {{old.*}}      - Previous property values
//   - {{new.*}}      - New property values
func InterpolateSafeOnly(template string, vars TemplateVars) string {
	if !strings.Contains(template, "{{") {
		return template
	}

	now := vars.Now()
	if vars.Now == nil {
		now = time.Now()
	}

	replacements := map[string]string{
		"{{now}}":        now.Format(time.RFC3339),
		"{{today}}":      now.Format("2006-01-02"),
		"{{user.name}}":  vars.User.Name,
		"{{user.email}}": vars.User.Email,
	}

	result := template
	for key, value := range replacements {
		result = strings.ReplaceAll(result, key, value)
	}

	return result
}

// interpolatePropertyRefs handles {{prefix.property}} patterns.
func interpolatePropertyRefs(s, prefix string, entity *model.Entity) string {
	marker := "{{" + prefix
	if !strings.Contains(s, marker) {
		return s
	}

	var buf bytes.Buffer
	remaining := s

	for {
		idx := strings.Index(remaining, marker)
		if idx == -1 {
			buf.WriteString(remaining)
			break
		}

		buf.WriteString(remaining[:idx])
		remaining = remaining[idx+len(marker):]

		// Find closing }}
		endIdx := strings.Index(remaining, "}}")
		if endIdx == -1 {
			buf.WriteString(marker)
			continue
		}

		propName := remaining[:endIdx]
		remaining = remaining[endIdx+2:]

		if entity != nil {
			buf.WriteString(entity.GetString(propName))
		}
		// If entity is nil, the variable expands to empty string
	}

	return buf.String()
}
