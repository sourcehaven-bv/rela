package rollup

import "strings"

// CountTasks counts the markdown task lines of body (TKT-SQ8O0EQ): list items
// whose text starts with a checkbox, `[ ]` open or `[x]`/`[X]` done. A list
// item starts with `-`, `*` or `+`, or with digits and `.` or `)`, followed
// by a space, at any indentation, so nested tasks count. Lines inside a
// fenced code block (``` or ~~~) do not count. An indented code block is
// not recognized: under a list item the same indentation is a nested task.
func CountTasks(body string) (done, total int) {
	var fence string // the open fence's marker run, "" outside a fence
	for line := range strings.Lines(body) {
		trimmed := strings.TrimLeft(strings.TrimRight(line, "\r\n"), " \t")
		if marker := fenceMarker(trimmed); marker != "" {
			switch {
			case fence == "":
				fence = marker
				continue
			case marker[0] == fence[0] && len(marker) >= len(fence) &&
				strings.TrimSpace(trimmed[len(marker):]) == "":
				fence = ""
				continue
			}
		}
		if fence != "" {
			continue
		}
		if checked, ok := taskItem(trimmed); ok {
			total++
			if checked {
				done++
			}
		}
	}
	return done, total
}

// fenceMarker is the run of three or more backticks or tildes that starts
// line, or "".
func fenceMarker(line string) string {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return ""
	}
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	if n < 3 {
		return ""
	}
	return line[:n]
}

// taskItem reports whether line, stripped of its indentation, is a list
// item starting with a checkbox, and whether the box is checked.
func taskItem(line string) (checked, ok bool) {
	rest, ok := listItemText(line)
	if !ok || len(rest) < 3 || rest[0] != '[' || rest[2] != ']' {
		return false, false
	}
	if len(rest) > 3 && rest[3] != ' ' && rest[3] != '\t' {
		return false, false // "[x]y" is text, not a checkbox
	}
	switch rest[1] {
	case ' ':
		return false, true
	case 'x', 'X':
		return true, true
	}
	return false, false
}

// listItemText is the text after a list marker and its space, and whether
// line starts with one.
func listItemText(line string) (string, bool) {
	if line == "" {
		return "", false
	}
	i := 0
	switch line[0] {
	case '-', '*', '+':
		i = 1
	default:
		for i < len(line) && i < 9 && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		if i == 0 || i >= len(line) || (line[i] != '.' && line[i] != ')') {
			return "", false
		}
		i++
	}
	if i >= len(line) || (line[i] != ' ' && line[i] != '\t') {
		return "", false
	}
	return strings.TrimLeft(line[i:], " \t"), true
}
