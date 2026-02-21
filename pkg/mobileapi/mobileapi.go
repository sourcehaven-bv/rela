// Package mobileapi provides an API for mobile clients to interact with rela projects.
// It wraps the dataentry HTTP handlers and exposes them via in-memory request/response
// calls, avoiding the need for a TCP server.
//
// This package is designed to be compiled with gomobile for iOS/Android:
//
//	gomobile bind -target=ios ./pkg/mobileapi
//
// The resulting framework can be called from Swift/Kotlin to interact with rela
// projects stored on the device.
package mobileapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var (
	mu      sync.RWMutex
	app     *dataentry.App
	handler http.Handler
)

// OpenProject initializes the mobile API with a rela project at the given path.
// The path should point to a directory containing metamodel.yaml.
// Returns an error if the project cannot be loaded.
func OpenProject(projectPath string) error {
	mu.Lock()
	defer mu.Unlock()

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	fs := storage.NewSafeFS(storage.NewOsFS())
	projCtx, err := project.Discover(absPath, fs)
	if err != nil {
		return fmt.Errorf("discovering project: %w", err)
	}

	repo := repository.New(fs, projCtx)
	newApp, err := dataentry.NewApp(repo)
	if err != nil {
		return fmt.Errorf("initializing app: %w", err)
	}

	app = newApp
	handler = app.NewRouter()

	return nil
}

// CloseProject releases resources associated with the current project.
func CloseProject() {
	mu.Lock()
	defer mu.Unlock()

	app = nil
	handler = nil
}

// IsProjectOpen returns true if a project is currently open.
func IsProjectOpen() bool {
	mu.RLock()
	defer mu.RUnlock()
	return app != nil
}

// ProjectName returns the name of the currently open project, or empty string if none.
func ProjectName() string {
	mu.RLock()
	defer mu.RUnlock()
	if app == nil {
		return ""
	}
	return app.ProjectName()
}

// ProjectRoot returns the root directory of the currently open project.
func ProjectRoot() string {
	mu.RLock()
	defer mu.RUnlock()
	if app == nil {
		return ""
	}
	return app.ProjectRoot()
}

// request performs an in-memory HTTP request against the dataentry handlers.
// This is an internal helper - use RequestJSON for mobile bindings.
func request(method, path, body string) (statusCode int, responseBody string, err error) {
	mu.RLock()
	defer mu.RUnlock()

	if handler == nil {
		return 0, "", errors.New("no project open")
	}

	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	result := rec.Result()
	defer result.Body.Close()

	respBytes, err := io.ReadAll(result.Body)
	if err != nil {
		return result.StatusCode, "", fmt.Errorf("reading response: %w", err)
	}

	return result.StatusCode, string(respBytes), nil
}

// RequestResult wraps the result of a Request call for languages that don't
// support multiple return values well.
type RequestResult struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
	Error      string `json:"error,omitempty"`
}

// RequestJSON performs an in-memory HTTP request and returns the result as JSON.
// This is the primary method for interacting with the API from mobile clients.
//
// Parameters:
//   - method: HTTP method (GET, POST, PUT, DELETE)
//   - path: Request path (e.g., "/api/entities?type=ticket")
//   - body: Request body (for POST/PUT requests), empty string for GET
//
// Returns a JSON object with fields: statusCode, body, error (if any).
func RequestJSON(method, path, body string) string {
	status, respBody, err := request(method, path, body)

	result := RequestResult{
		StatusCode: status,
		Body:       respBody,
	}
	if err != nil {
		result.Error = err.Error()
	}

	out, _ := json.Marshal(result)
	return string(out)
}

// --- Convenience methods for common operations ---

// ListEntityTypes returns JSON array of available entity types.
func ListEntityTypes() (string, error) {
	status, body, err := request("GET", "/api/entity-types", "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", status, body)
	}
	return body, nil
}

// ListEntities returns JSON array of entities of the given type.
func ListEntities(entityType string) (string, error) {
	path := "/api/entities?type=" + entityType
	status, body, err := request("GET", path, "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", status, body)
	}
	return body, nil
}

// GetEntity returns JSON representation of a single entity.
func GetEntity(entityID string) (string, error) {
	path := "/api/entities/" + entityID
	status, body, err := request("GET", path, "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", status, body)
	}
	return body, nil
}

// GetMetamodel returns the project's metamodel as JSON.
func GetMetamodel() (string, error) {
	status, body, err := request("GET", "/api/metamodel", "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", status, body)
	}
	return body, nil
}

// Analyze returns validation/analysis results as JSON.
func Analyze() (string, error) {
	status, body, err := request("GET", "/api/analyze", "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", status, body)
	}
	return body, nil
}

// Search returns entities matching the given query as JSON.
func Search(query string) (string, error) {
	path := "/api/search?q=" + query
	status, body, err := request("GET", path, "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", status, body)
	}
	return body, nil
}

// AllEntities returns all entities regardless of type as JSON.
func AllEntities() (string, error) {
	status, body, err := request("GET", "/api/entities", "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", status, body)
	}
	return body, nil
}
