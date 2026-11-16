package apptest_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// caseTrigger reads the trigger name a case covers: `t.case("<trigger>: ...")`.
var caseTrigger = regexp.MustCompile(`(?m)^t\.case\("([a-z0-9-]+):`)

// TestTicketsTriggersCovered pins the dogfood of the tickets checklist
// workflow (TKT-XB4M29R): every trigger in tickets/flows.yaml has a case in
// tickets/tests/checklist-workflow.test.lua named after it, and every case
// prefix other than "edge" names a trigger that exists. CI runs the suite with
// `rela test --require-tests`.
func TestTicketsTriggersCovered(t *testing.T) {
	raw, err := os.ReadFile("../../tickets/flows.yaml")
	require.NoError(t, err)
	var file struct {
		Triggers map[string]yaml.Node `yaml:"triggers"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &file))
	require.NotEmpty(t, file.Triggers)

	suite, err := os.ReadFile("../../tickets/tests/checklist-workflow.test.lua")
	require.NoError(t, err)
	covered := map[string]bool{}
	for _, m := range caseTrigger.FindAllStringSubmatch(string(suite), -1) {
		covered[m[1]] = true
	}
	require.NotEmpty(t, covered, "the suite has no cases")

	for name := range file.Triggers {
		if !covered[name] {
			t.Errorf("trigger %q in tickets/flows.yaml has no case named %q in the suite", name, name+": ...")
		}
	}
	for name := range covered {
		if _, ok := file.Triggers[name]; !ok && name != "edge" {
			t.Errorf("case prefix %q names no trigger in tickets/flows.yaml", name)
		}
	}
}
