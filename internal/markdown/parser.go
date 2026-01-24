package markdown

import (
	"bufio"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterDelimiter = "---"

// Document represents a parsed markdown document with YAML frontmatter
type Document struct {
	Frontmatter map[string]interface{}
	Content     string
}

// ParseDocument parses a markdown document with YAML frontmatter
func ParseDocument(content string) (*Document, error) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var fm map[string]interface{}
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	return &Document{
		Frontmatter: fm,
		Content:     body,
	}, nil
}

// splitFrontmatter separates YAML frontmatter from markdown content
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []string
	inFrontmatter := false
	frontmatterEnded := false
	frontmatterLines := []string{}

	for scanner.Scan() {
		line := scanner.Text()

		if !inFrontmatter && !frontmatterEnded && strings.TrimSpace(line) == frontmatterDelimiter {
			inFrontmatter = true
			continue
		}

		if inFrontmatter && strings.TrimSpace(line) == frontmatterDelimiter {
			inFrontmatter = false
			frontmatterEnded = true
			continue
		}

		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		} else if frontmatterEnded || !inFrontmatter {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	frontmatter = strings.Join(frontmatterLines, "\n")
	body = strings.TrimPrefix(strings.Join(lines, "\n"), "\n")

	return frontmatter, body, nil
}

// FormatDocument formats a document back to markdown with YAML frontmatter
func FormatDocument(frontmatter map[string]interface{}, content string) (string, error) {
	var sb strings.Builder

	if len(frontmatter) > 0 {
		sb.WriteString(frontmatterDelimiter)
		sb.WriteString("\n")

		yamlBytes, err := yaml.Marshal(frontmatter)
		if err != nil {
			return "", err
		}
		sb.Write(yamlBytes)
		sb.WriteString(frontmatterDelimiter)
		sb.WriteString("\n")
	}

	if content != "" {
		sb.WriteString("\n")
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// GetString extracts a string value from frontmatter
func (d *Document) GetString(key string) string {
	if d.Frontmatter == nil {
		return ""
	}
	if v, ok := d.Frontmatter[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetStringSlice extracts a string slice from frontmatter
func (d *Document) GetStringSlice(key string) []string {
	if d.Frontmatter == nil {
		return nil
	}
	if v, ok := d.Frontmatter[key]; ok {
		switch val := v.(type) {
		case []interface{}:
			result := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		case []string:
			return val
		}
	}
	return nil
}
