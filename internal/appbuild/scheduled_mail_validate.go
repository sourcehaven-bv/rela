package appbuild

import (
	"context"
	"fmt"
	stdmail "net/mail"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/mailtemplate"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ValidateScheduledMailRecipients checks current graph addresses selected by
// template tasks. Validation is operator-scoped and therefore scans the raw
// store; rendering remains recipient ACL-scoped at execution.
func (s *Services) ValidateScheduledMailRecipients(ctx context.Context) error {
	templateData, err := s.cfgLoader.Load(ctx, mailtemplate.ConfigFile)
	if err != nil {
		return err
	}
	templates, err := mailtemplate.Parse(templateData, s.meta)
	if err != nil {
		return err
	}
	scheduleData, err := s.cfgLoader.Load(ctx, scheduler.ConfigFile)
	if err != nil {
		return err
	}
	schedules, err := scheduler.ParseConfig(scheduleData)
	if err != nil {
		return err
	}

	var findings []string
	for _, task := range schedules.Tasks {
		if task.Template == "" {
			continue
		}
		tmpl, ok := templates.Templates[task.Template]
		if !ok {
			continue
		} // Named by structural validation before this pass.
		def, _ := s.meta.GetEntityDef(task.ForEach.EntityType)
		filters, _ := filter.ParseAll(task.ForEach.Where)
		var invalid []string
		for ent, listErr := range s.store.ListEntities(ctx, store.EntityQuery{Type: task.ForEach.EntityType}) {
			if listErr != nil {
				return listErr
			}
			matched, matchErr := filter.MatchAll(filter.Record{
				ID: ent.ID, Type: ent.Type, Properties: ent.Properties, ModifiedAt: ent.UpdatedAt,
			}, filters, def, s.meta)
			if matchErr != nil {
				return matchErr
			}
			if matched && !usableAddress(ent.Properties[tmpl.AddressProperty]) {
				invalid = append(invalid, ent.ID)
			}
		}
		if len(invalid) > 0 {
			sort.Strings(invalid)
			findings = append(findings, fmt.Sprintf("task %q property %q invalid on %s",
				task.Name, tmpl.AddressProperty, strings.Join(invalid, ", ")))
		}
	}
	if len(findings) > 0 {
		return fmt.Errorf("%s", strings.Join(findings, "; "))
	}
	return nil
}

func usableAddress(value any) bool {
	raw, ok := value.(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return false
	}
	_, err := stdmail.ParseAddress(raw)
	return err == nil
}
