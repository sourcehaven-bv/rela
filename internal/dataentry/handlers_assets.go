package dataentry

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// AssetInfo represents an attachment with usage information.
type AssetInfo struct {
	Path          string
	DisplayName   string
	ContentType   string
	Size          int64
	SizeFormatted string
	Added         time.Time
	AddedBy       string
	IsImage       bool
	IsOrphan      bool
	UsedBy        []AssetUsage
}

// AssetUsage tracks where an attachment is used.
type AssetUsage struct {
	EntityType string
	EntityID   string
	Property   string
	Title      string
}

// handleAssets renders the asset management page.
func (a *App) handleAssets(w http.ResponseWriter, r *http.Request) {
	store := attachment.NewStore(a.repo.FS(), a.repo.Paths().Root)
	attachments, err := store.ListWithMetadata()
	if err != nil {
		http.Error(w, "Failed to list attachments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build reference map from graph
	refs := a.buildAttachmentReferences()

	// Convert to AssetInfo with usage data
	assets := make([]AssetInfo, 0, len(attachments))
	var totalSize int64
	var orphanCount int

	for _, att := range attachments {
		displayName := att.Path
		contentType := "application/octet-stream"
		var size int64
		var added time.Time
		var addedBy string

		if att.Metadata != nil {
			if att.Metadata.OriginalName != "" {
				displayName = att.Metadata.OriginalName
			}
			if att.Metadata.ContentType != "" {
				contentType = att.Metadata.ContentType
			}
			size = att.Metadata.Size
			added = att.Metadata.Added
			addedBy = att.Metadata.AddedBy
		}

		usedBy := refs[att.Path]
		isOrphan := len(usedBy) == 0

		totalSize += size
		if isOrphan {
			orphanCount++
		}

		assets = append(assets, AssetInfo{
			Path:          att.Path,
			DisplayName:   displayName,
			ContentType:   contentType,
			Size:          size,
			SizeFormatted: attachment.FormatSize(size),
			Added:         added,
			AddedBy:       addedBy,
			IsImage:       strings.HasPrefix(contentType, "image/"),
			IsOrphan:      isOrphan,
			UsedBy:        usedBy,
		})
	}

	// Sort by display name
	sort.Slice(assets, func(i, j int) bool {
		return strings.ToLower(assets[i].DisplayName) < strings.ToLower(assets[j].DisplayName)
	})

	// Check for filter
	filter := r.URL.Query().Get("filter")
	if filter == "orphans" {
		filtered := make([]AssetInfo, 0)
		for _, asset := range assets {
			if asset.IsOrphan {
				filtered = append(filtered, asset)
			}
		}
		assets = filtered
	}

	data := map[string]interface{}{
		"App":         a.Cfg.App,
		"Navigation":  a.navElements("_assets"),
		"ActiveList":  "_assets",
		"Assets":      assets,
		"TotalCount":  len(attachments),
		"TotalSize":   attachment.FormatSize(totalSize),
		"OrphanCount": orphanCount,
		"Filter":      filter,
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "assets-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "assets-page", data) //nolint:errcheck // template errors logged by http
	}
}

// buildAttachmentReferences scans the graph to find all attachment references.
func (a *App) buildAttachmentReferences() map[string][]AssetUsage {
	refs := make(map[string][]AssetUsage)

	// Find all file properties in metamodel
	fileProps := make(map[string][]string) // entityType -> []propertyName
	for typeName, entDef := range a.meta.Entities {
		for propName, prop := range entDef.Properties {
			if prop.Type == metamodel.PropertyTypeFile {
				fileProps[typeName] = append(fileProps[typeName], propName)
			}
		}
	}

	// Scan all entities for file property values
	for typeName, propNames := range fileProps {
		entities := a.g.NodesByType(typeName)
		for _, e := range entities {
			title := a.entityDisplayTitle(e)
			for _, propName := range propNames {
				paths := getFilePaths(e, propName)
				for _, path := range paths {
					refs[path] = append(refs[path], AssetUsage{
						EntityType: typeName,
						EntityID:   e.ID,
						Property:   propName,
						Title:      title,
					})
				}
			}
		}
	}

	return refs
}

// getFilePaths extracts file paths from an entity property (handles single and multiple).
func getFilePaths(e *model.Entity, propName string) []string {
	val, ok := e.Properties[propName]
	if !ok || val == nil {
		return nil
	}

	switch v := val.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []interface{}:
		paths := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				paths = append(paths, s)
			}
		}
		return paths
	case []string:
		paths := make([]string, 0, len(v))
		for _, s := range v {
			if s != "" {
				paths = append(paths, s)
			}
		}
		return paths
	}

	return nil
}

// handleAssetRename updates an attachment's display name.
func (a *App) handleAssetRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.FormValue("path")
	newName := r.FormValue("name")

	if path == "" || newName == "" {
		http.Error(w, "Missing path or name", http.StatusBadRequest)
		return
	}

	store := attachment.NewStore(a.repo.FS(), a.repo.Paths().Root)
	if err := store.UpdateDisplayName(path, newName); err != nil {
		http.Error(w, "Failed to update name: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return JSON success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // response write error
		"success": true,
		"name":    newName,
	})
}

// handleAssetDelete removes an orphaned attachment.
func (a *App) handleAssetDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.FormValue("path")
	if path == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	// Verify it's orphaned before allowing delete
	refs := a.buildAttachmentReferences()
	if len(refs[path]) > 0 {
		http.Error(w, "Cannot delete: attachment is still in use", http.StatusBadRequest)
		return
	}

	store := attachment.NewStore(a.repo.FS(), a.repo.Paths().Root)
	if err := store.Remove(path); err != nil {
		http.Error(w, "Failed to delete: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return JSON success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // response write error
		"success": true,
	})
}

// handleAssetGC removes all orphaned attachments.
func (a *App) handleAssetGC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all referenced paths
	refs := a.buildAttachmentReferences()
	referencedPaths := make([]string, 0, len(refs))
	for path := range refs {
		referencedPaths = append(referencedPaths, path)
	}

	store := attachment.NewStore(a.repo.FS(), a.repo.Paths().Root)
	result, err := store.GC(referencedPaths)
	if err != nil {
		http.Error(w, "GC failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Actually remove the files
	if err := store.RemoveUnreferenced(result); err != nil {
		http.Error(w, "Remove failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return JSON with results
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // response write error
		"success":   true,
		"removed":   len(result.Removed),
		"reclaimed": attachment.FormatSize(result.Reclaimed),
	})
}

// handleAssetList returns assets as JSON for the asset picker.
func (a *App) handleAssetList(w http.ResponseWriter, _ *http.Request) {
	store := attachment.NewStore(a.repo.FS(), a.repo.Paths().Root)
	attachments, err := store.ListWithMetadata()
	if err != nil {
		http.Error(w, "Failed to list attachments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type assetJSON struct {
		Path        string `json:"path"`
		DisplayName string `json:"displayName"`
		ContentType string `json:"contentType"`
		Size        string `json:"size"`
		IsImage     bool   `json:"isImage"`
	}

	assets := make([]assetJSON, 0, len(attachments))
	for _, att := range attachments {
		displayName := att.Path
		contentType := "application/octet-stream"
		var size int64

		if att.Metadata != nil {
			if att.Metadata.OriginalName != "" {
				displayName = att.Metadata.OriginalName
			}
			if att.Metadata.ContentType != "" {
				contentType = att.Metadata.ContentType
			}
			size = att.Metadata.Size
		}

		assets = append(assets, assetJSON{
			Path:        att.Path,
			DisplayName: displayName,
			ContentType: contentType,
			Size:        attachment.FormatSize(size),
			IsImage:     strings.HasPrefix(contentType, "image/"),
		})
	}

	// Sort by display name
	sort.Slice(assets, func(i, j int) bool {
		return strings.ToLower(assets[i].DisplayName) < strings.ToLower(assets[j].DisplayName)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assets) //nolint:errcheck // response write error
}

// handleAssetPreview returns asset details for the preview modal.
func (a *App) handleAssetPreview(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	store := attachment.NewStore(a.repo.FS(), a.repo.Paths().Root)
	meta, _ := store.GetMetadata(path)

	displayName := path
	contentType := "application/octet-stream"
	var size int64
	var added time.Time
	var addedBy string

	if meta != nil {
		if meta.OriginalName != "" {
			displayName = meta.OriginalName
		}
		if meta.ContentType != "" {
			contentType = meta.ContentType
		}
		size = meta.Size
		added = meta.Added
		addedBy = meta.AddedBy
	}

	refs := a.buildAttachmentReferences()
	usedBy := refs[path]

	data := map[string]interface{}{
		"Path":        path,
		"DisplayName": displayName,
		"ContentType": contentType,
		"Size":        attachment.FormatSize(size),
		"Added":       added.Format("2006-01-02 15:04"),
		"AddedBy":     addedBy,
		"IsImage":     strings.HasPrefix(contentType, "image/"),
		"IsOrphan":    len(usedBy) == 0,
		"UsedBy":      usedBy,
	}

	a.tmpl.ExecuteTemplate(w, "asset-preview", data) //nolint:errcheck // template errors logged by http
}

// handleFileUpload handles file uploads and returns the attachment path.
func (a *App) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with 32MB max memory
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file contents
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Store the attachment
	store := attachment.NewStore(a.repo.FS(), a.repo.Paths().Root)
	att, err := store.AddBytes(data, header.Filename, "web-upload")
	if err != nil {
		http.Error(w, "Failed to store attachment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return JSON with the attachment path
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // response write error
		"success":     true,
		"path":        att.Path,
		"displayName": header.Filename,
	})
}

// handleAttachmentServe serves attachment files from the attachments directory.
func (a *App) handleAttachmentServe(w http.ResponseWriter, r *http.Request) {
	// Extract path - remove leading slash only, keep "attachments/" prefix
	// since store paths are relative to project root (e.g., "attachments/ab/hash.png")
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || !strings.HasPrefix(path, "attachments/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	store := attachment.NewStore(a.repo.FS(), a.repo.Paths().Root)

	// Get metadata for content type
	meta, _ := store.GetMetadata(path)
	contentType := "application/octet-stream"
	displayName := path
	if meta != nil {
		if meta.ContentType != "" {
			contentType = meta.ContentType
		}
		if meta.OriginalName != "" {
			displayName = meta.OriginalName
		}
	}

	// Read the file
	data, err := store.Get(path)
	if err != nil {
		http.Error(w, "File not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+filepath.Base(displayName)+"\"")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // Content-addressed, can cache forever
	w.Write(data)                                                          //nolint:errcheck // response write error
}
