package triggers

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TemplateVars holds variables available for template interpolation.
type TemplateVars struct {
	// Now is the current time function (injectable for testing).
	Now func() time.Time

	// User holds user identity information.
	User UserVars

	// CurrentUserID is the user entity the acting principal resolved to,
	// bound to {{current_user.id}}. Empty when the write has no resolved
	// user entity. [Engine.Process] sets it per event from ctx.
	CurrentUserID string

	// currentUser enables {{current_user.id}}. Only the trigger engine sets
	// it: copy fields and conversion templates also call [Interpolate], and
	// there the placeholder stays literal text as before.
	currentUser bool
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

	const gitConfigTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), gitConfigTimeout)
	defer cancel()

	if name, err := gitConfig(ctx, "user.name"); err == nil {
		vars.Name = name
	}

	if email, err := gitConfig(ctx, "user.email"); err == nil {
		vars.Email = email
	}

	return vars
}

// gitConfig reads one git config key, with the environment every child
// process gets (cmdexec.Environ).
func gitConfig(ctx context.Context, key string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "config", key)
	cmd.Env = cmdexec.Environ()
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Interpolate replaces template variables in a string.
// Supported variables:
//   - {{now}}         - Current timestamp (ISO 8601)
//   - {{today}}       - Current date (YYYY-MM-DD)
//   - {{user.name}}   - Git user name
//   - {{user.email}}  - Git user email
//   - {{entity.id}}   - Current entity ID
//   - {{entity.type}} - Current entity type
//   - {{current_user.id}} - User entity of the acting principal, or empty
//   - {{old.<prop>}}  - Previous value of a property
//   - {{new.<prop>}}  - New value of a property
//
// Every placeholder is replaced in one pass, and substituted text is never
// scanned again: a stored value that holds `{{new.x}}` is written as that
// literal text. An unknown placeholder is kept as written.
func Interpolate(template string, vars TemplateVars, entity, oldEntity *entity.Entity) string {
	if !strings.Contains(template, "{{") {
		return template
	}
	safe := safeVars(vars)
	return expand(template, func(key string) (string, bool) {
		if v, ok := safe[key]; ok {
			return v, true
		}
		switch {
		case key == "entity.id" && entity != nil:
			return entity.ID, true
		case key == "entity.type" && entity != nil:
			return entity.Type, true
		case key == PlaceholderCurrentUserID && vars.currentUser:
			return vars.CurrentUserID, true
		case strings.HasPrefix(key, "old."):
			return propertyValue(oldEntity, strings.TrimPrefix(key, "old.")), true
		case strings.HasPrefix(key, "new."):
			return propertyValue(entity, strings.TrimPrefix(key, "new.")), true
		}
		return "", false
	})
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
//   - {{current_user.id}} - The acting user (scripts read rela.principal)
//   - {{old.*}}      - Previous property values
//   - {{new.*}}      - New property values
func InterpolateSafeOnly(template string, vars TemplateVars) string {
	if !strings.Contains(template, "{{") {
		return template
	}
	safe := safeVars(vars)
	return expand(template, func(key string) (string, bool) {
		v, ok := safe[key]
		return v, ok
	})
}

// safeVars returns the placeholders that carry no entity data, keyed
// without braces.
func safeVars(vars TemplateVars) map[string]string {
	// The nil check comes FIRST. It used to follow the call, which panicked
	// on a zero TemplateVars — latent because the automation engine always
	// populates Now, so no caller reached it until the copy kernel passed a
	// bare TemplateVars (TKT-C1XUA8).
	now := time.Now()
	if vars.Now != nil {
		now = vars.Now()
	}
	return map[string]string{
		"now":        now.Format(time.RFC3339),
		"today":      now.Format("2006-01-02"),
		"user.name":  vars.User.Name,
		"user.email": vars.User.Email,
	}
}

// expand replaces each `{{key}}` in s for which lookup answers, left to
// right, and copies everything else as written, including an unclosed `{{`.
func expand(s string, lookup func(key string) (string, bool)) string {
	var buf strings.Builder
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			buf.WriteString(s)
			return buf.String()
		}
		j := strings.Index(s[i+2:], "}}")
		if j < 0 {
			buf.WriteString(s)
			return buf.String()
		}
		buf.WriteString(s[:i])
		key := s[i+2 : i+2+j]
		if v, ok := lookup(key); ok {
			buf.WriteString(v)
		} else {
			buf.WriteString(s[i : i+2+j+2])
		}
		s = s[i+2+j+2:]
	}
}

// PropertyReads returns the properties the `{{new.x}}` and `{{old.x}}`
// placeholders of s read, in order, found by the same scan [Interpolate]
// expands them with.
func PropertyReads(s string) []string {
	var out []string
	expand(s, func(key string) (string, bool) {
		for _, prefix := range []string{"new.", "old."} {
			if p, ok := strings.CutPrefix(key, prefix); ok {
				out = append(out, p)
			}
		}
		return "", false
	})
	return out
}

// propertyValue is property name of e as a string, or "" when e is nil.
func propertyValue(e *entity.Entity, name string) string {
	if e == nil {
		return ""
	}
	return e.GetString(name)
}
