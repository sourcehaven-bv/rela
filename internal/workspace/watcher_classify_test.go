package workspace

import (
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestClassifyDataEvents(t *testing.T) {
	viewsPath := filepath.Clean("/project/views.yaml")
	entitiesDir := filepath.Clean("/project/entities")
	relationsDir := filepath.Clean("/project/relations")

	tests := []struct {
		name   string
		events []storage.ChangeEvent
		want   reloadAction
	}{
		{
			name:   "entity change triggers sync",
			events: []storage.ChangeEvent{{Path: "/project/entities/requirement/REQ-001.md", Op: storage.OpModify}},
			want:   actionSync,
		},
		{
			name:   "relation change triggers sync",
			events: []storage.ChangeEvent{{Path: "/project/relations/REQ-001--implements--FEAT-001.md", Op: storage.OpModify}},
			want:   actionSync,
		},
		{
			name:   "views change triggers notify only",
			events: []storage.ChangeEvent{{Path: "/project/views.yaml", Op: storage.OpModify}},
			want:   actionNotify,
		},
		{
			name: "mixed entity and views triggers sync",
			events: []storage.ChangeEvent{
				{Path: "/project/entities/requirement/REQ-001.md", Op: storage.OpModify},
				{Path: "/project/views.yaml", Op: storage.OpModify},
			},
			want: actionSync,
		},
		{
			name:   "unknown file triggers notify",
			events: []storage.ChangeEvent{{Path: "/project/something-else.yaml", Op: storage.OpModify}},
			want:   actionNotify,
		},
		{
			name:   "empty events triggers notify",
			events: []storage.ChangeEvent{},
			want:   actionNotify,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyDataEvents(tt.events, viewsPath, entitiesDir, relationsDir)
			if got != tt.want {
				t.Errorf("classifyDataEvents() = %d, want %d", got, tt.want)
			}
		})
	}
}
