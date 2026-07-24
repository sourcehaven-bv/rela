package transform

import "github.com/Sourcehaven-BV/rela/internal/metamodel"

// RegistryFromMetamodel projects the metamodel's validated `transforms:` map
// into a [Registry]. The metamodel loader has already validated and
// CANONICALIZED each entry (non-empty command, from == markdown — an empty
// `from` is written back as markdown at load — well-formed produces), so this
// is a pure shape conversion. An empty or absent map yields an empty (non-nil)
// registry.
func RegistryFromMetamodel(m *metamodel.Metamodel) Registry {
	reg := make(Registry, len(m.Transforms))
	for name, def := range m.Transforms {
		reg[name] = Def{
			From:     def.From,
			Command:  def.Command,
			Produces: def.Produces,
		}
	}
	return reg
}
