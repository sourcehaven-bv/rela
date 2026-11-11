package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// channelFileRunners holds the command runner of channel file scans and
// transforms. Only the data-entry server installs one, after it checked
// that the sandbox does not expose the project
// ([SetChannelFileRunner]). Until then, and in the CLI and stdio MCP,
// there is none: native MIME checks still run, and a policy that needs a
// command refuses every file, as an entity upload does there.
type channelFileRunners struct {
	r atomic.Pointer[runnerBox]
}

// runnerBox lets an interface value sit behind an atomic pointer.
type runnerBox struct{ attachment.CommandRunner }

// runner returns the installed runner.
//
// Nil: returned when none is installed, or h is nil.
func (h *channelFileRunners) runner() attachment.CommandRunner {
	if h == nil {
		return nil
	}
	if box := h.r.Load(); box != nil {
		return box.CommandRunner
	}
	return nil
}

// SetChannelFileRunner installs the runner of channel file scans and
// transforms for the channel service of s. The data-entry host calls it
// with the runner its app built. A package function because Services is
// at its plimsoll cap.
//
// Nil: r rejected; without a runner, install nothing.
func SetChannelFileRunner(s *Services, r attachment.CommandRunner) error {
	if r == nil {
		return errors.New("appbuild: SetChannelFileRunner: runner is required")
	}
	if s.base == nil || s.base.channelFiles == nil {
		return errors.New("appbuild: SetChannelFileRunner: services were not fully assembled")
	}
	s.base.channelFiles.r.Store(&runnerBox{r})
	return nil
}

// scanCannotRunWarning keeps [warnIfChannelScanCannotRun] to one line per
// process.
var scanCannotRunWarning sync.Once

// warnIfChannelScanCannotRun logs, once, that a channel file property
// configures a scan this process cannot run, so every file posted to it
// is refused. The channel config reloads at runtime, so this is judged on
// the first such post rather than at boot, as the entity upload path does.
func warnIfChannelScanCannotRun(pol attachment.FilePolicy, runner attachment.CommandRunner) {
	if len(pol.ScanCmd) == 0 {
		return
	}
	var why error
	switch r := runner.(type) {
	case nil:
		why = errors.New("this process runs no external commands")
	case interface{ SandboxErr() error }:
		why = r.SandboxErr()
	}
	if why != nil {
		scanCannotRunWarning.Do(func() {
			slog.Warn("channels: a file property configures a virus scan that cannot run here, "+
				"so EVERY file posted to it is refused", "err", why, "docs", "docs/attachment-security.md")
		})
	}
}

// channelFileChecker adapts the attachment pipeline to
// [realtime.FileChecker]: the property's policy resolved against the
// project's global attachment settings, then [attachment.Checker.Check].
// A refusal is recorded in the audit log, as an entity upload's is.
type channelFileChecker struct {
	meta    *metamodel.Metamodel
	runners *channelFileRunners
	audit   audit.Audit
}

// newChannelFileChecker returns the checker over meta's global attachment
// settings and the runner runners holds, recording refusals in sink.
//
// Nil: meta accepted (no global settings apply); runners accepted (no
// runner); sink accepted for an assembly without an audit log, whose
// records are discarded.
func newChannelFileChecker(
	meta *metamodel.Metamodel, runners *channelFileRunners, sink audit.Audit,
) channelFileChecker {
	if sink == nil {
		sink = audit.Nop{}
	}
	return channelFileChecker{meta: meta, runners: runners, audit: sink}
}

func (c channelFileChecker) CheckFile(ctx context.Context, f realtime.FileCheck) (realtime.CheckedFile, error) {
	name := channelFileName(f.Name)
	pol := attachment.PolicyFor(c.meta, f.Policy)
	runner := c.runners.runner()
	warnIfChannelScanCannotRun(pol, runner)
	got, err := attachment.NewChecker(runner).Check(ctx, pol, attachment.ProcessContext{
		EntityID: f.Channel.ID, Property: f.Property, FileName: name,
	}, f.Data)
	if errors.Is(err, attachment.ErrRejected) {
		reason := attachment.RejectionReason(err)
		c.audit.Record(audit.ChannelFileRejected(ctx, f.Channel.ID, f.Channel.String(), f.Property, name, reason))
		return realtime.CheckedFile{}, fmt.Errorf("%w: %s", realtime.ErrFileRefused, reason)
	}
	if err != nil {
		return realtime.CheckedFile{}, err
	}
	return realtime.CheckedFile{Name: channelFileName(got.FileName), MIME: got.MIME, Data: got.Data}, nil
}

// channelFileName is the display name of a posted file: the entity
// upload's normalization, then every control character the channel store
// refuses replaced.
func channelFileName(raw string) string {
	name := attachment.DisplayName(strings.ToValidUTF8(raw, "_"))
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return '_'
		}
		return r
	}, name)
}
