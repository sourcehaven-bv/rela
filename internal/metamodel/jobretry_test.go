package metamodel

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestJobRetry_Unmarshal(t *testing.T) {
	for in, want := range map[string]JobRetry{"never": JobRetryNever, "Bounded": JobRetryBounded, "persistent": JobRetryPersistent} {
		var r JobRetry
		if err := yaml.Unmarshal([]byte(`"`+in+`"`), &r); err != nil || r != want {
			t.Errorf("%q: got %q, %v", in, r, err)
		}
	}
	var r JobRetry
	if err := yaml.Unmarshal([]byte("forever"), &r); err == nil {
		t.Error("unknown retry accepted")
	}
}
