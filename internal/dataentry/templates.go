package dataentry

import (
	"io/fs"
	"strings"
)

// allTemplates reads all embedded .html template files and concatenates them
// into a single string for parsing by html/template.
func allTemplates() string {
	var sb strings.Builder
	err := fs.WalkDir(templateFiles, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		data, err := templateFiles.ReadFile(path)
		if err != nil {
			return err
		}
		sb.Write(data)
		sb.WriteByte('\n')
		return nil
	})
	if err != nil {
		panic("failed to read embedded templates: " + err.Error())
	}
	return sb.String()
}
