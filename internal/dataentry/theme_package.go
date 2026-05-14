package dataentry

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// ThemePackageMaxBytes caps the on-the-wire size of a `.relatheme`
// upload. The manifest is at most a couple of KB; a logo is ≤256 KiB.
// 5 MiB leaves generous headroom while bounding the worst-case memory
// pressure a single import can impose.
const ThemePackageMaxBytes = 5 << 20

// themePackageMaxExpansion bounds the ratio of declared uncompressed
// size to actual zip size. Defends against zip-bomb entries that look
// small on the wire but expand into many MB. 100× is far above any
// honest text-or-image ratio (PNG/JPEG/WebP barely compress; YAML
// compresses ~3-5×).
const themePackageMaxExpansion = 100

// themeManifestEntry is the canonical name of the manifest file in the
// archive. Required.
const themeManifestEntry = "theme.yaml"

// themeLogoPrefix is the canonical prefix of the optional logo entry.
// The full entry name is `logo.<ext>` where <ext> matches the manifest's
// `logo:` field.
const themeLogoPrefix = "logo."

// ImportedThemeAsset carries the bytes + sniffed extension of a logo
// extracted from a theme package. ImportedThemeAsset is nil when the
// package contained no logo (or didn't reference one).
type ImportedThemeAsset struct {
	Bytes []byte
	Ext   string
}

// ParsedThemePackage is the typed result of parsing a `.relatheme`
// upload. Manifest is always populated on success; Logo is non-nil only
// when the manifest referenced a logo and the bytes passed validation.
type ParsedThemePackage struct {
	Manifest *dataentryconfig.ThemeManifest
	Logo     *ImportedThemeAsset
}

// Sentinel errors for parseThemePackage. Callers translate these into
// HTTP status codes; the messages are user-facing.
var (
	errNotAZip          = errors.New("not a valid zip file")
	errMissingManifest  = errors.New("missing theme.yaml in archive")
	errInvalidManifest  = errors.New("invalid theme manifest")
	errMissingLogo      = errors.New("logo referenced in manifest but not present in archive")
	errLogoTooLarge     = errors.New("logo exceeds size limit")
	errZipPathTraversal = errors.New("zip entry contains path traversal")
	errZipBomb          = errors.New("zip declared expansion ratio exceeds limit")
	errZipUncompressed  = errors.New("zip uncompressed total exceeds limit")
	errDuplicateEntry   = errors.New("zip contains duplicate entry")
)

// parseThemePackage interprets the bytes of a `.relatheme` upload. It
// performs all validation in-memory (no filesystem access) so it's
// trivially unit-testable as a pure function.
//
// Validation order is deliberately layered: cheap structural checks
// (size, zip parse, entry names) before any decompression of bodies,
// so a hostile upload is rejected without paying the full cost.
//
// yaml.v3 panics rather than returning an error if struct tags
// duplicate a top-level key. ThemeManifest's reflection sanity-check
// in package init catches that at startup, but the recover here means
// a future regression surfaces as a 400 rather than a process crash.
func parseThemePackage(raw []byte) (pkg *ParsedThemePackage, err error) {
	defer func() {
		if r := recover(); r != nil {
			pkg = nil
			err = fmt.Errorf("%w: panic during parse: %v", errInvalidManifest, r)
		}
	}()
	return parseThemePackageImpl(raw)
}

func parseThemePackageImpl(raw []byte) (*ParsedThemePackage, error) {
	if len(raw) > ThemePackageMaxBytes {
		return nil, errZipUncompressed
	}

	zr, zerr := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if zerr != nil {
		return nil, fmt.Errorf("%w: %w", errNotAZip, zerr)
	}

	// Defense-in-depth: catch zip-bombs before we ever read entries.
	if err := checkZipExpansion(zr, int64(len(raw))); err != nil {
		return nil, err
	}

	// Index entries by name for direct lookup. We only honor
	// flat-filename entries that match our allowlist; everything else
	// (READMEs, .DS_Store, future additions) is silently ignored.
	entries := make(map[string]*zip.File)
	for _, f := range zr.File {
		name := f.Name
		// Reject anything with path-like syntax. Zip stores POSIX-ish
		// names; backslashes are still suspicious on any platform.
		if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
			return nil, fmt.Errorf("%w: %q", errZipPathTraversal, name)
		}
		// Skip directory entries (technically still flat after the
		// slash check, but be explicit).
		if f.FileInfo().IsDir() {
			continue
		}
		// Reject duplicate names. A polyglot zip with two `theme.yaml`
		// or two `logo.<ext>` entries would otherwise let an attacker
		// ship one set of bytes for inspection-via-iteration and a
		// different set for our map-lookup-driven import.
		if _, dup := entries[name]; dup {
			return nil, fmt.Errorf("%w: %q", errDuplicateEntry, name)
		}
		entries[name] = f
	}

	manifestFile, ok := entries[themeManifestEntry]
	if !ok {
		return nil, errMissingManifest
	}

	manifestBytes, mErr := readZipEntry(manifestFile, ThemePackageMaxBytes)
	if mErr != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidManifest, mErr)
	}

	var manifest dataentryconfig.ThemeManifest
	if err := yaml.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidManifest, err)
	}
	if err := dataentryconfig.ValidateThemeManifest(&manifest); err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidManifest, err)
	}

	out := &ParsedThemePackage{Manifest: &manifest}

	if manifest.Logo != "" {
		logoFile, ok := entries[manifest.Logo]
		if !ok {
			return nil, errMissingLogo
		}
		logoBytes, err := readZipEntry(logoFile, MaxUserLogoBytes+1)
		if err != nil {
			return nil, fmt.Errorf("logo: %w", err)
		}
		if len(logoBytes) > MaxUserLogoBytes {
			return nil, errLogoTooLarge
		}
		// Trust the actual bytes, not the manifest's claimed extension.
		// sniffLogoMime is the same trust boundary the direct PUT path
		// uses; reusing it keeps both paths in lockstep.
		mime := sniffLogoMime(logoBytes)
		ext := logoExtForMime(mime)
		if ext == "" {
			return nil, fmt.Errorf("logo: unsupported format %q (accepted: image/png, image/jpeg, image/svg+xml, image/webp)", mime)
		}
		out.Logo = &ImportedThemeAsset{Bytes: logoBytes, Ext: ext}
	}

	return out, nil
}

// checkZipExpansion guards against zip-bomb attacks. It sums the
// declared uncompressed sizes (bounded against overflow) and rejects
// archives whose sum exceeds either the absolute cap or a fixed ratio
// of the input size. Decompression itself is bounded again by
// readZipEntry's LimitReader; this is the cheap layer that runs first.
func checkZipExpansion(zr *zip.Reader, compressedTotal int64) error {
	const maxRatioInput = (1 << 63) / themePackageMaxExpansion // guard the multiply
	var total uint64
	for _, f := range zr.File {
		// Saturating add: any entry that would push us over the cap
		// (including a single attacker-controlled zip64 size near
		// 2^64) is rejected before total wraps.
		if f.UncompressedSize64 > ThemePackageMaxBytes ||
			total > ThemePackageMaxBytes-f.UncompressedSize64 {

			return errZipUncompressed
		}
		total += f.UncompressedSize64
	}
	// compressedTotal comes from len([]byte) of the upload, so it's
	// bounded by ThemePackageMaxBytes — well under the multiply cap.
	if compressedTotal < 0 || compressedTotal > maxRatioInput {
		return errZipBomb
	}
	if total > uint64(compressedTotal)*themePackageMaxExpansion {
		return errZipBomb
	}
	return nil
}

// readZipEntry reads up to limit+1 bytes from a zip entry, returning an
// error if the entry exceeds limit. The +1 lets the caller distinguish
// "exactly at limit" (accept) from "over limit" (reject) with a single
// length comparison.
func readZipEntry(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(io.LimitReader(rc, limit+1))
}
