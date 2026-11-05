package migration

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&NamespaceToIRIMigration{})
}

// NamespaceToIRIMigration renames the root `namespace:` key, which held an
// ontology IRI, to `iri:`. `namespace:` now names the prefix of an included
// schema file (TKT-C91W7H4). Detect sees one document and cannot tell a
// root file from an included one, so it moves only an IRI-shaped value (one
// containing ":"), which a namespace name cannot be. The loader itself reads
// any root value as the IRI.
type NamespaceToIRIMigration struct{}

func (m *NamespaceToIRIMigration) Name() string { return "namespace-to-iri" }

// Advisory reports true: the loader still reads a root `namespace:` that
// holds an IRI, so a project need not migrate before it loads.
func (m *NamespaceToIRIMigration) Advisory() bool { return true }

func (m *NamespaceToIRIMigration) Description() string {
	return "Rename the root `namespace:` key (an ontology IRI) to `iri:`"
}

func (m *NamespaceToIRIMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel}
}

func (m *NamespaceToIRIMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}
	ns := GetMapValue(root, "namespace")
	return ns != nil && ns.Kind == yaml.ScalarNode && strings.Contains(ns.Value, ":") &&
		GetMapKey(root, "iri") == nil
}

func (m *NamespaceToIRIMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return errors.New("empty document")
	}
	RenameMapKey(root, "namespace", "iri")
	return nil
}
