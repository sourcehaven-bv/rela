package workspace

import (
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/repository"
)

func TestClassifyEvents(t *testing.T) {
	schemaFiles := []string{
		filepath.Clean("/project/metamodel.yaml"),
		filepath.Clean("/project/types.yaml"),
	}
	viewsPath := filepath.Clean("/project/views.yaml")
	entitiesDir := filepath.Clean("/project/entities")
	relationsDir := filepath.Clean("/project/relations")

	tests := []struct {
		name   string
		events []repository.ChangeEvent
		want   reloadAction
	}{
		{
			name:   "metamodel change triggers reload",
			events: []repository.ChangeEvent{{Path: "/project/metamodel.yaml", Op: repository.OpModify}},
			want:   actionReload,
		},
		{
			name:   "include file change triggers reload",
			events: []repository.ChangeEvent{{Path: "/project/types.yaml", Op: repository.OpModify}},
			want:   actionReload,
		},
		{
			name:   "entity change triggers sync",
			events: []repository.ChangeEvent{{Path: "/project/entities/requirement/REQ-001.md", Op: repository.OpModify}},
			want:   actionSync,
		},
		{
			name:   "relation change triggers sync",
			events: []repository.ChangeEvent{{Path: "/project/relations/REQ-001--implements--FEAT-001.md", Op: repository.OpModify}},
			want:   actionSync,
		},
		{
			name:   "views change triggers notify only",
			events: []repository.ChangeEvent{{Path: "/project/views.yaml", Op: repository.OpModify}},
			want:   actionNotify,
		},
		{
			name: "mixed metamodel and entity triggers reload",
			events: []repository.ChangeEvent{
				{Path: "/project/entities/requirement/REQ-001.md", Op: repository.OpModify},
				{Path: "/project/metamodel.yaml", Op: repository.OpModify},
			},
			want: actionReload,
		},
		{
			name: "mixed entity and views triggers sync",
			events: []repository.ChangeEvent{
				{Path: "/project/entities/requirement/REQ-001.md", Op: repository.OpModify},
				{Path: "/project/views.yaml", Op: repository.OpModify},
			},
			want: actionSync,
		},
		{
			name:   "unknown file triggers notify",
			events: []repository.ChangeEvent{{Path: "/project/something-else.yaml", Op: repository.OpModify}},
			want:   actionNotify,
		},
		{
			name:   "empty events triggers notify",
			events: []repository.ChangeEvent{},
			want:   actionNotify,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyEvents(tt.events, schemaFiles, viewsPath, entitiesDir, relationsDir)
			if got != tt.want {
				t.Errorf("classifyEvents() = %d, want %d", got, tt.want)
			}
		})
	}
}
