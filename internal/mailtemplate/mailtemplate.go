// Package mailtemplate builds recipient-scoped mail models from project declarations.
package mailtemplate

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/mailrender"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const ConfigFile = "mail-templates.yaml"

type Config struct {
	Templates map[string]Template `yaml:"mail_templates"`
}

type Template struct {
	Subject         string `yaml:"subject"`
	Intro           string `yaml:"intro,omitempty"`
	AddressProperty string `yaml:"address_property"`

	// RequireVisibleContent suppresses the send entirely when no section
	// received content for this recipient. Off by default: a template may
	// legitimately carry a meaningful Intro with no matching entities.
	//
	// The decision uses [Build]'s contributed count, not its matched count —
	// see that function's doc for why the two differ.
	RequireVisibleContent bool `yaml:"require_visible_content,omitempty"`

	Sections []Section `yaml:"sections,omitempty"`
}

type Section struct {
	Title      string   `yaml:"title,omitempty"`
	EntityType string   `yaml:"entity_type"`
	Where      []string `yaml:"where,omitempty"`
	Style      string   `yaml:"style,omitempty"`
	Columns    []string `yaml:"columns,omitempty"`
	Link       bool     `yaml:"link,omitempty"`
}

type Reader interface {
	ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

func Parse(data []byte, meta *metamodel.Metamodel) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ConfigFile, err)
	}
	for name, tmpl := range cfg.Templates {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(tmpl.Subject) == "" || strings.TrimSpace(tmpl.AddressProperty) == "" {
			return nil, fmt.Errorf("mail template %q: subject and address_property are required", name)
		}
		// Sections are otherwise optional — a template may be pure intro. But
		// require_visible_content asks "send only when a section has content",
		// which a template with no sections can never satisfy: it would parse,
		// validate, schedule, and then silently discard every send forever.
		// Refuse the contradiction at load rather than run it.
		if tmpl.RequireVisibleContent && len(tmpl.Sections) == 0 {
			return nil, fmt.Errorf(
				"mail template %q: require_visible_content needs at least one section, "+
					"otherwise no mail can ever be sent", name)
		}
		for i, section := range tmpl.Sections {
			def, ok := meta.GetEntityDef(section.EntityType)
			if !ok {
				return nil, fmt.Errorf("mail template %q section %d: unknown entity type %q", name, i, section.EntityType)
			}
			if section.Style != "" && section.Style != "table" && section.Style != "list" && section.Style != "detail" {
				return nil, fmt.Errorf("mail template %q section %d: unknown style %q", name, i, section.Style)
			}
			if _, err := filter.ParseAll(section.Where); err != nil {
				return nil, fmt.Errorf("mail template %q section %d where: %w", name, i, err)
			}
			for _, column := range section.Columns {
				if _, ok := def.Properties[column]; !ok {
					return nil, fmt.Errorf("mail template %q section %d: unknown property %q", name, i, column)
				}
			}
		}
	}
	return &cfg, nil
}

// Build assembles a recipient-scoped message and reports how many entities
// contributed content to it.
//
// The returned count is NOT the number of entities that matched. A matched
// entity contributes nothing when its section renders it as nothing — a
// `detail` section whose entity has empty Content is the case that matters,
// since it produces the "Nothing to show." placeholder that
// [Template.RequireVisibleContent] exists to suppress. Matches are still
// counted separately because `{{count}}` interpolates them.
//
// Emptiness is decided here rather than by inspecting the returned Message
// because each style stores its content in a different field (Body for
// detail/list, Rows for table), so a predicate over the rendered message
// would need revisiting for every style added later.
func Build(
	ctx context.Context, meta *metamodel.Metamodel, reader Reader, tmpl Template, now time.Time,
) (*mailrender.Message, int, error) {
	msg := &mailrender.Message{}
	count := 0
	contributed := 0
	for _, declared := range tmpl.Sections {
		section, tally, err := buildSection(ctx, meta, reader, declared)
		if err != nil {
			return nil, 0, err
		}
		count += tally.matched
		contributed += tally.contributed
		msg.Sections = append(msg.Sections, section)
	}
	msg.Subject = expand(tmpl.Subject, now, count)
	msg.Intro = expand(tmpl.Intro, now, count)
	return msg, contributed, nil
}

// sectionTally separates the two counts one section produces. They diverge
// only for `detail` — see [Build].
type sectionTally struct {
	matched     int
	contributed int
}

func buildSection(
	ctx context.Context, meta *metamodel.Metamodel, reader Reader, declared Section,
) (mailrender.Section, sectionTally, error) {
	def, _ := meta.GetEntityDef(declared.EntityType)
	filters, _ := filter.ParseAll(declared.Where)
	section := mailrender.Section{Title: declared.Title}
	if declared.Style == "table" || declared.Style == "" {
		section.Columns = append([]string(nil), declared.Columns...)
	}
	var tally sectionTally
	for ent, err := range reader.ListEntities(ctx, store.EntityQuery{Type: declared.EntityType}) {
		if err != nil {
			return mailrender.Section{}, sectionTally{}, err
		}
		record := filter.Record{
			ID: ent.ID, Type: ent.Type, Properties: ent.Properties, ModifiedAt: ent.UpdatedAt,
		}
		matched, err := filter.MatchAll(record, filters, def, meta)
		if err != nil {
			return mailrender.Section{}, sectionTally{}, err
		}
		if !matched {
			continue
		}
		tally.matched++
		if appendEntity(&section, declared, ent) {
			tally.contributed++
		}
	}
	return section, tally, nil
}

// appendEntity adds one matched entity to the section in the declared style,
// reporting whether it contributed any content. Only `detail` can decline:
// an entity with a blank body renders as nothing.
func appendEntity(section *mailrender.Section, declared Section, ent *entity.Entity) bool {
	switch declared.Style {
	case "detail":
		if strings.TrimSpace(ent.Content) == "" {
			return false
		}
		if section.Body != "" {
			section.Body += "\n\n"
		}
		section.Body += ent.Content
	case "list":
		section.Body += fmt.Sprintf("- [%s](/entity/%s/%s)\n", entityLabel(ent), ent.Type, ent.ID)
	default:
		row := make([]string, len(declared.Columns))
		for i, column := range declared.Columns {
			row[i] = fmt.Sprint(ent.Properties[column])
		}
		section.Rows = append(section.Rows, row)
		if declared.Link {
			section.Links = append(section.Links, "/entity/"+ent.Type+"/"+ent.ID)
		}
	}
	return true
}

func entityLabel(ent *entity.Entity) string {
	for _, key := range []string{"title", "name"} {
		if value := strings.TrimSpace(fmt.Sprint(ent.Properties[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ent.ID
}

func expand(value string, now time.Time, count int) string {
	value = strings.ReplaceAll(value, "{{today}}", now.Format(time.DateOnly))
	return strings.ReplaceAll(value, "{{count}}", strconv.Itoa(count))
}
