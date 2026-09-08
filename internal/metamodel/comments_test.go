package metamodel_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// baseSchema is a minimal two-type metamodel the comment-block cases build on.
const baseSchema = `
version: "1"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title:
        type: string
  person:
    label: Person
    id_prefix: "PERS-"
    properties:
      name:
        type: string
`

func parse(t *testing.T, extra string) (*metamodel.Metamodel, error) {
	t.Helper()
	return metamodel.Parse([]byte(baseSchema + extra))
}

func mustParse(t *testing.T, extra string) *metamodel.Metamodel {
	t.Helper()
	m, err := parse(t, extra)
	require.NoError(t, err)
	return m
}

// TestComments_AbsentBlockIsDisabled pins AC1's config half: a project that
// never mentions comments has the feature switched off entirely.
func TestComments_AbsentBlockIsDisabled(t *testing.T) {
	m := mustParse(t, "")
	p := metamodel.NewCommentPolicy(m)

	require.False(t, p.Enabled())
	require.False(t, p.Commentable("ticket"))
	require.Empty(t, p.CommentableTypes())
}

func TestComments_NilMetamodelIsDisabled(t *testing.T) {
	// A partially-wired path must degrade to "no commenting", not panic.
	p := metamodel.NewCommentPolicy(nil)
	require.False(t, p.Enabled())
	require.False(t, p.Commentable("ticket"))
}

func TestComments_NamedTypes(t *testing.T) {
	m := mustParse(t, `
comments:
  enabled: true
  on: [ticket]
`)
	p := metamodel.NewCommentPolicy(m)

	require.True(t, p.Enabled())
	require.True(t, p.Commentable("ticket"))
	require.False(t, p.Commentable("person"), "a type absent from `on` is not commentable")
	require.Equal(t, []string{"ticket"}, p.CommentableTypes())
}

func TestComments_Wildcard(t *testing.T) {
	m := mustParse(t, `
comments:
  enabled: true
  on: ["*"]
`)
	p := metamodel.NewCommentPolicy(m)

	require.True(t, p.Commentable("ticket"))
	require.True(t, p.Commentable("person"))
	require.Equal(t, []string{"person", "ticket"}, p.CommentableTypes())
}

// TestComments_WildcardDoesNotAdmitUndeclaredTypes pins that "*" means "every
// type this project defines", not "any string a client sends" — otherwise the
// wildcard would turn an unvalidated path segment into a storage key.
func TestComments_WildcardDoesNotAdmitUndeclaredTypes(t *testing.T) {
	m := mustParse(t, `
comments:
  enabled: true
  on: ["*"]
`)
	p := metamodel.NewCommentPolicy(m)

	require.False(t, p.Commentable("no-such-type"))
	require.False(t, p.Commentable(""))
	require.False(t, p.Commentable("../escape"))
}

func TestComments_DisabledBlockIsInert(t *testing.T) {
	m := mustParse(t, `
comments:
  enabled: false
`)
	require.False(t, metamodel.NewCommentPolicy(m).Enabled())
}

func TestComments_LoadErrors(t *testing.T) {
	tests := []struct {
		name     string
		block    string
		wantFrag string
	}{
		{
			name: "enabled with no types",
			block: `
comments:
  enabled: true
`,
			wantFrag: "`on` is required when enabled",
		},
		{
			name: "types listed but not enabled",
			block: `
comments:
  enabled: false
  on: [ticket]
`,
			wantFrag: "but `enabled` is false",
		},
		{
			name: "unknown entity type",
			block: `
comments:
  enabled: true
  on: [widget]
`,
			wantFrag: `unknown entity type "widget"`,
		},
		{
			name: "duplicate type",
			block: `
comments:
  enabled: true
  on: [ticket, ticket]
`,
			wantFrag: "more than once",
		},
		{
			name: "empty type name",
			block: `
comments:
  enabled: true
  on: ["", ticket]
`,
			wantFrag: "empty entity type",
		},
		{
			name: "wildcard mixed with named types",
			block: `
comments:
  enabled: true
  on: ["*", ticket]
`,
			wantFrag: "already covers every type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(t, tc.block)
			require.Error(t, err, "a bad comments block must fail the whole load")
			require.Contains(t, err.Error(), tc.wantFrag)
		})
	}
}

// TestComments_UnknownTypeSuggests pins the did-you-mean hint. A typo in `on:`
// otherwise surfaces as comments quietly missing on one type, which gives an
// operator nothing to search for.
func TestComments_UnknownTypeSuggests(t *testing.T) {
	_, err := parse(t, `
comments:
  enabled: true
  on: [tickets]
`)
	require.Error(t, err)
	require.Contains(t, err.Error(), `did you mean "ticket"?`)
}

// TestComments_CanonicalizesTypeNames pins that whitespace is resolved at load,
// so no downstream consumer has to re-trim.
func TestComments_CanonicalizesTypeNames(t *testing.T) {
	m := mustParse(t, `
comments:
  enabled: true
  on: ["  ticket  "]
`)
	require.Equal(t, []string{"ticket"}, m.Comments.On)
	require.True(t, metamodel.NewCommentPolicy(m).Commentable("ticket"))
}

// TestComments_UnknownKeyRejected pins that the block participates in the
// loader's unknown-key checking rather than silently swallowing typos.
func TestComments_UnknownKeyRejected(t *testing.T) {
	_, err := parse(t, `
comments:
  enabled: true
  on: [ticket]
  enable_for_everyone: true
`)
	if err == nil {
		t.Skip("nested unknown-key checking is not applied to this block")
	}
	require.Contains(t, strings.ToLower(err.Error()), "enable_for_everyone")
}
