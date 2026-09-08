package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/perfseed"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// DevCmd groups developer tooling that is not part of operating a project.
// Each subcommand says in its help what it bypasses.
type DevCmd struct {
	Seed DevSeedCmd `cmd:"" help:"Fill an EMPTY store with generated data for performance work (raw store write, no automations or validation)."`
}

// DevSeedCmd is `rela dev seed`: it loads the perfseed graph into the
// project's store — markdown files on the default build, PostgreSQL on the
// postgres build — so the data-entry app can be measured at scale.
//
// This is the fourth sanctioned raw-store write path (after `db migrate`,
// `history-purge` and data migration) and carries the same obligations: the
// trust boundary is the operator shell, every write is attributed
// (store.WithAttribution) and the run leaves one audit record. It refuses a
// store that already holds entities unless --force is given, because the
// generator mints fixed ids and a second run would collide with or sit
// beside real data.
type DevSeedCmd struct {
	Profile string  `default:"perf" enum:"perf" help:"Which graph to generate (only 'perf' today: prototypes/perf/project)."`
	Scale   float64 `default:"1" help:"Size multiplier; 1 is roughly 20k entities and 45k relations."`
	Seed    uint64  `default:"1" help:"PRNG seed; the same seed and scale always produce the same graph."`
	Batch   int     `default:"500" help:"Writes per transaction."`
	Force   bool    `help:"Seed even though the store already holds entities. The generated rows then sit beside the real ones; they carry tool attribution 'perf-seed' and the run's audit record, which is how to find and remove them."`
}

// seedTool is the attribution tool name stamped on every seeded row and the
// audit record's op, so seeded data is distinguishable from real edits.
const seedTool = "perf-seed"

// Run executes `rela dev seed`.
func (c *DevSeedCmd) Run(ctx context.Context, write *writeServices) error {
	if c.Scale <= 0 || c.Scale > 10 {
		return errors.New("--scale must be in (0, 10]")
	}
	if c.Batch <= 0 {
		return errors.New("--batch must be positive")
	}
	existing, err := write.Store.CountEntities(ctx, store.EntityQuery{})
	if err != nil {
		return fmt.Errorf("count entities: %w", err)
	}
	if existing > 0 && !c.Force {
		return fmt.Errorf("store already holds %d entities; seeding needs an empty store (or --force)", existing)
	}

	p := principal.From(ctx)
	user := p.User
	if user == "" {
		user = principal.ReservedPrefix + seedTool
	}
	ctx = store.WithAttribution(ctx, store.Attribution{User: user, Tool: seedTool})

	gen := perfseed.New(perfseed.Perf(c.Scale), c.Seed)
	prof := gen.Profile()
	out.WriteMessage("Seeding profile %s at scale %g (seed %d): %d teams, %d people, %d projects, %d tasks, "+
		"%d controls, %d risks, %d policies, %d documents",
		c.Profile, c.Scale, c.Seed, prof.Teams, prof.People, prof.Projects, prof.Tasks,
		prof.Controls, prof.Risks, prof.Policies, prof.Documents)

	last := time.Now()
	sum, err := perfseed.Load(ctx, write.Store, gen, perfseed.LoadOptions{
		BatchSize: c.Batch,
		Progress: func(s perfseed.Summary) {
			if time.Since(last) < 2*time.Second {
				return
			}
			last = time.Now()
			out.WriteMessage("  %d entities, %d relations (%s)", s.Entities, s.Relations, s.Elapsed.Round(time.Second))
		},
	})
	summary := fmt.Sprintf("perf seed %s scale=%g seed=%d: %d entities, %d relations in %s",
		c.Profile, c.Scale, c.Seed, sum.Entities, sum.Relations, sum.Elapsed.Round(time.Millisecond))
	if err != nil {
		summary += " (FAILED: " + err.Error() + ")"
	}
	if write.Audit != nil {
		write.Audit.Record(audit.Record{
			Time:      time.Now().UTC(),
			Op:        audit.OpPerfSeed,
			Principal: p,
			Summary:   summary,
		})
	}
	if err != nil {
		return fmt.Errorf("seed: %w (wrote %d entities, %d relations before failing)", err, sum.Entities, sum.Relations)
	}
	out.WriteSuccess("%s", summary)
	return nil
}
