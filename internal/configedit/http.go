package configedit

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// maxDraftBytes bounds a request body. rela's own configuration is about
// 200 KB as JSON; this leaves ample room.
const maxDraftBytes = 8 << 20

// Handler serves the Configure API. Authorization is the mounting site's job:
// it must only be reachable by a principal holding config:edit.
//
//	GET  /           the files as trees, with their version
//	POST /preview    check a draft and describe the save
//	POST /save       save a draft
//	POST /migrate    finish a migration a save left incomplete
func (s *Service) Handler(prefix string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+prefix, func(w http.ResponseWriter, _ *http.Request) {
		snap, err := s.Snapshot()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, snap)
	})
	mux.HandleFunc("POST "+prefix+"/preview", func(w http.ResponseWriter, r *http.Request) {
		d, ok := readDraft(w, r)
		if !ok {
			return
		}
		res, err := s.Preview(r.Context(), d)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})
	mux.HandleFunc("POST "+prefix+"/save", func(w http.ResponseWriter, r *http.Request) {
		d, ok := readDraft(w, r)
		if !ok {
			return
		}
		res, err := s.Save(r.Context(), d)
		if err != nil {
			writeError(w, err)
			return
		}
		status := http.StatusOK
		if !res.Saved {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, res)
	})
	mux.HandleFunc("POST "+prefix+"/migrate", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Retry(r.Context()); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func readDraft(w http.ResponseWriter, r *http.Request) (*Draft, bool) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxDraftBytes))
	dec.DisallowUnknownFields()
	var d Draft
	if err := dec.Decode(&d); err != nil {
		writeProblem(w, http.StatusBadRequest, "bad_request", "The request body is not a valid draft.")
		return nil, false
	}
	return &d, true
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrConflict):
		writeProblem(w, http.StatusConflict, "conflict",
			"The configuration changed since you opened it. Reload to see the current version.")
	case errors.Is(err, ErrBusy):
		writeProblem(w, http.StatusConflict, "busy",
			"A data migration is running or records are being saved. Try again shortly.")
	case errors.Is(err, ErrMigrationNotStarted):
		slog.Error("configedit: migration did not start", "error", err)
		writeProblem(w, http.StatusConflict, "migration_not_started",
			"The data migration could not start. The configuration was not changed.")
	case errors.Is(err, ErrBadDraft):
		writeProblem(w, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, ErrUnsupported):
		writeProblem(w, http.StatusUnprocessableEntity, "unsupported", err.Error())
	default:
		slog.Error("configedit: request failed", "error", err)
		writeProblem(w, http.StatusInternalServerError, "internal", "The configuration could not be saved.")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, status int, kind, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "https://rela.dev/errors/" + kind, "title": detail, "status": status,
	})
}
