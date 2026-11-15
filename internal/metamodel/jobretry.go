package metamodel

import (
	"fmt"
	"strings"
)

// JobRetry is a background automation job's retry intent (TKT-2Q4UFI). It
// names the intent only; attempts and backoff belong to internal/jobs.
type JobRetry string

const (
	// JobRetryDefault is the zero value and means JobRetryBounded.
	JobRetryDefault    JobRetry = ""
	JobRetryNever      JobRetry = "never"
	JobRetryBounded    JobRetry = "bounded"
	JobRetryPersistent JobRetry = "persistent"
)

// UnmarshalYAML accepts only the three intents.
func (r *JobRetry) UnmarshalYAML(unmarshal func(any) error) error {
	var raw string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch v := JobRetry(strings.ToLower(strings.TrimSpace(raw))); v {
	case JobRetryDefault, JobRetryNever, JobRetryBounded, JobRetryPersistent:
		*r = v
		return nil
	default:
		return fmt.Errorf("invalid retry value %q (want %q, %q or %q)",
			raw, JobRetryNever, JobRetryBounded, JobRetryPersistent)
	}
}
