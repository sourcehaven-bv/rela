package migration

import (
	"bytes"
	"fmt"
	"log/slog"
	"slices"

	"gopkg.in/yaml.v3"
)

// Migrated is the result of [MigrateBytes].
type Migrated struct {
	// Data is the migrated content, or the input itself when nothing applied.
	Data []byte
	// Applied lists the migrations that rewrote Data, in application order.
	Applied []DetectionResult
}

// Explain adds to err, an error about Data, that its line numbers refer to
// the migrated form. It returns err unchanged when nothing was applied.
func (m Migrated) Explain(err error) error {
	if err == nil || len(m.Applied) == 0 {
		return err
	}
	return fmt.Errorf("%w (line numbers refer to the file after in-memory migration; run 'rela migrate')", err)
}

// MigrateBytes applies the pending migrations for ft to data in memory, so a
// loader can read old syntax as `rela migrate` would rewrite it. Nothing is
// written. It repeats until nothing is pending, so a migration that enables
// another is followed. Advisory migrations are skipped: their syntax loads.
// A project-level or [ReviewRequired] migration returns *Error, as does
// input that is still pending after one pass per registered migration.
// Data that is not valid YAML comes back unchanged for the parser to report.
func MigrateBytes(path string, data []byte, ft FileType, meta MetamodelProvider) (Migrated, error) {
	unchanged := Migrated{Data: data}
	doc, parsed := parseNode(data)
	if !parsed {
		return unchanged, nil
	}
	var applied []DetectionResult
	for range len(ForFileType(ft)) + 1 {
		pending := Blocking(DetectFromNodeWithMetamodel(doc, ft, meta))
		if len(pending) == 0 {
			return encodeMigrated(path, doc, applied, unchanged)
		}
		if refused := refusedOnLoad(pending); len(refused) > 0 {
			return Migrated{}, &Error{FilePath: path, Detections: refused}
		}
		for _, d := range pending {
			if err := d.Migration.Apply(doc); err != nil {
				return Migrated{}, fmt.Errorf("%s: migration %s failed: %w", path, d.Migration.Name(), err)
			}
		}
		applied = append(applied, pending...)
	}
	return Migrated{}, &Error{FilePath: path, Detections: Blocking(DetectFromNodeWithMetamodel(doc, ft, meta))}
}

// encodeMigrated renders doc after the applied migrations, or returns
// unchanged when none applied.
func encodeMigrated(path string, doc *yaml.Node, applied []DetectionResult, unchanged Migrated) (Migrated, error) {
	if len(applied) == 0 {
		return unchanged, nil
	}
	out, err := encodeNode(doc)
	if err != nil { // coverage-ignore: defensive: a parsed *yaml.Node always encodes into a bytes.Buffer
		return Migrated{}, fmt.Errorf("%s: %w", path, err)
	}
	return Migrated{Data: out, Applied: slices.Clip(applied)}, nil
}

// parseNode parses data, reporting false when it is not valid YAML.
func parseNode(data []byte) (*yaml.Node, bool) {
	var doc yaml.Node
	return &doc, yaml.Unmarshal(data, &doc) == nil
}

// refusedOnLoad returns the detections a loader may not apply in memory:
// those that span files and those that need a person's review.
func refusedOnLoad(detections []DetectionResult) []DetectionResult {
	var out []DetectionResult
	for _, d := range detections {
		if IsProjectLevel(d.Migration) || IsReviewRequired(d.Migration) {
			out = append(out, d)
		}
	}
	return out
}

// encodeNode renders doc the way `rela migrate` writes a migrated file.
func encodeNode(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil { // coverage-ignore: defensive: see encodeMigrated
		return nil, fmt.Errorf("encoding YAML: %w", err)
	}
	if err := encoder.Close(); err != nil { // coverage-ignore: defensive: Close flushes into a bytes.Buffer
		return nil, fmt.Errorf("closing encoder: %w", err)
	}
	return refoldFolded(buf.Bytes()), nil
}

// WarnApplied logs one warning for a file that loaded through [MigrateBytes]
// with migrations applied. It logs nothing when applied is empty.
func WarnApplied(path string, applied []DetectionResult) {
	if len(applied) == 0 {
		return
	}
	names := make([]string, len(applied))
	for i, d := range applied {
		names[i] = d.Migration.Name()
	}
	slog.Warn("config uses deprecated syntax; loaded it with in-memory migrations. "+
		"Run 'rela migrate' to update the file",
		"file", path, "migrations", names)
}
