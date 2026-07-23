package transform

import "github.com/Sourcehaven-BV/rela/internal/metamodel"

// RegistryFromMetamodel projects the metamodel's validated `transforms:` map
// into a [Registry]. The metamodel loader has already validated each entry
// (non-empty command, from == markdown, well-formed produces), so this is a
// pure shape conversion. An empty or absent map yields an empty (non-nil)
// registry.
func RegistryFromMetamodel(m *metamodel.Metamodel) Registry {
	reg := make(Registry, len(m.Transforms))
	for name, def := range m.Transforms {
		from := def.From
		if from == "" {
			from = FormatMarkdown
		}
		reg[name] = Def{
			From:     from,
			Command:  def.Command,
			Produces: def.Produces,
		}
	}
	return reg
}
