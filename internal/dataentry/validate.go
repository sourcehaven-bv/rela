package dataentry

import "github.com/Sourcehaven-BV/rela/internal/dataentryconfig"

// ValidateConfig re-exports the validation function from dataentryconfig.
var ValidateConfig = dataentryconfig.ValidateConfig

// CollectConfigWarnings re-exports the non-fatal config-warning collector.
var CollectConfigWarnings = dataentryconfig.CollectConfigWarnings
