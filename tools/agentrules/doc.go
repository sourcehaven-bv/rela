// Package agentrules holds the guard test for the path-scoped agent rules in
// .claude/rules. Each rule file names the files it applies to in a `paths:`
// frontmatter list, and Claude Code loads the rule when one of them is read
// or edited. A glob that matches nothing unloads the rule without any error,
// so the test fails instead.
package agentrules
