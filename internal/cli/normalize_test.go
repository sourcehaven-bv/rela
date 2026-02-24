package cli

import "testing"

func TestNormalizeCmd_DryRunFlagExists(t *testing.T) {
	flag := normalizeCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Error("normalize command should have --dry-run flag")
	}
}
