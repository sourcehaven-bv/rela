package autocascade

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// BackgroundScripts schedules a `background: true` script action
// (TKT-2Q4UFI) instead of running it in the cascade. The wiring site
// supplies it over the job queue; this package may not import jobs.
type BackgroundScripts interface {
	EnqueueScript(ctx context.Context, s BackgroundScript) error
}

// BackgroundScript names one background action and the entity whose save
// triggered it. It carries no identity or capability: the implementation
// takes those from the action's configuration.
type BackgroundScript struct {
	Automation string
	LuaFile    string
	Ref        entity.Ref
}
