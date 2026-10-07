package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Sourcehaven-BV/rela/internal/fsimport"
)

func TestPrintImportReport(t *testing.T) {
	var buf bytes.Buffer
	printImportReport(&buf, &fsimport.Report{
		Source:   "/a",
		Target:   "/b",
		Entities: 2,
		Skipped: []fsimport.Skip{
			{Path: "entities/x/y.md", Reason: "folder"},
			{Path: "entities/x/\x1b[2Kz.md", Reason: "folder"},
		},
		GitCrypt: true,
	}, nil)
	out := buf.String()
	assert.Contains(t, out, "entities/x/y.md: folder")
	assert.Contains(t, out, "entities/x/\uFFFD[2Kz.md", "control characters are replaced")
	assert.NotContains(t, out, "\x1b")
	assert.Contains(t, out, "UNENCRYPTED")
	assert.Contains(t, out, "credential-name")

	buf.Reset()
	printImportReport(&buf, &fsimport.Report{Errors: []string{"bad"}}, assert.AnError)
	assert.Contains(t, buf.String(), "did not complete")
	assert.NotContains(t, buf.String(), "Next steps")
}
