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
	Subject         string    `yaml:"subject"`
	Intro           string    `yaml:"intro,omitempty"`
	AddressProperty string    `yaml:"address_property"`
	Sections        []Section `yaml:"sections,omitempty"`
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

func Build(
	ctx context.Context, meta *metamodel.Metamodel, reader Reader, tmpl Template, now time.Time,
) (*mailrender.Message, error) {
	msg := &mailrender.Message{}
	count := 0
	for _, declared := range tmpl.Sections {
		def, _ := meta.GetEntityDef(declared.EntityType)
		filters, _ := filter.ParseAll(declared.Where)
		section := mailrender.Section{Title: declared.Title}
		if declared.Style == "table" || declared.Style == "" {
			section.Columns = append([]string(nil), declared.Columns...)
		}
		for ent, err := range reader.ListEntities(ctx, store.EntityQuery{Type: declared.EntityType}) {
			if err != nil {
				return nil, err
			}
			record := filter.Record{
				ID: ent.ID, Type: ent.Type, Properties: ent.Properties, ModifiedAt: ent.UpdatedAt,
			}
			matched, err := filter.MatchAll(record, filters, def, meta)
			if err != nil {
				return nil, err
			}
			if !matched {
				continue
			}
			count++
			switch declared.Style {
			case "detail":
				if section.Body != "" {
					section.Body += "\n\n"
				}
				section.Body += ent.Content
			case "list":
				label := entityLabel(ent)
				section.Body += fmt.Sprintf("- [%s](/entity/%s/%s)\n", label, ent.Type, ent.ID)
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
		}
		msg.Sections = append(msg.Sections, section)
	}
	msg.Subject = expand(tmpl.Subject, now, count)
	msg.Intro = expand(tmpl.Intro, now, count)
	return msg, nil
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
