package appbuild

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// ValidationModeEnv overrides the `validation:` key of .rela/config.yaml.
const ValidationModeEnv = "RELA_VALIDATION"

// validationModeFor resolves the deployment's validation mode (DEC-OYV2AK,
// TKT-TXTMB0): $RELA_VALIDATION when set, else `validation:` in
// .rela/config.yaml, else advisory.
//
// It is a deployment setting, not a schema one, which is why it is not in
// schema.yaml: two deployments of one schema may differ, as a filesystem
// checkout edited by hand and a managed server do. .rela/ is not committed,
// and the environment variable suits a container. An unknown value fails
// the boot rather than falling back to advisory, because a deployment that
// asked to be strict must not run open.
func validationModeFor(cfg Config) (entitymanager.ValidationMode, error) {
	if v := strings.TrimSpace(os.Getenv(ValidationModeEnv)); v != "" {
		mode, err := entitymanager.ParseValidationMode(v)
		if err != nil {
			return mode, fmt.Errorf("$%s: %w", ValidationModeEnv, err)
		}
		return mode, nil
	}
	if cfg.Paths == nil || cfg.Paths.CacheDir == "" {
		return entitymanager.ValidationAdvisory, nil
	}
	pc, err := project.LoadConfigFromPath(cfg.Paths.ConfigPath())
	if err != nil {
		return entitymanager.ValidationAdvisory, fmt.Errorf("read %s: %w", cfg.Paths.ConfigPath(), err)
	}
	mode, err := entitymanager.ParseValidationMode(pc.Validation)
	if err != nil {
		return mode, fmt.Errorf("%s: %w", cfg.Paths.ConfigPath(), err)
	}
	return mode, nil
}
