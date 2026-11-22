package appbuild

import (
	"context"
	"iter"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// blindRows serves no entity: every subject is withheld from the reader.
type blindRows struct{}

func (blindRows) GetAddress(context.Context, string) (*entity.Entity, error) {
	return nil, store.ErrNotFound
}

func (blindRows) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(func(*entity.Entity, error) bool) {}
}

func (blindRows) ListRelationsStrict(context.Context, store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	return func(func(*entity.Relation, error) bool) {}
}

func (blindRows) ResolveHeadersErr(context.Context, []entity.Ref) (map[entity.Ref]visibility.ResolvedHeader, error) {
	return map[entity.Ref]visibility.ResolvedHeader{}, nil
}

func failedJobView() flowview.Run {
	job := func() *flowview.Waiting {
		return &flowview.Waiting{Pending: &flowview.Pending{Job: &flowview.Job{Phase: "failed", Failed: 1, Error: "salary 9000"}}}
	}
	return flowview.Run{Waiting: job(), Parallel: []flowview.Place{{Branch: "b", Waiting: job()}}}
}

// TestWithholdJobError pins that the job error, which can hold subject
// values, reaches only a caller who manages the run and reads its subject,
// in every branch.
func TestWithholdJobError(t *testing.T) {
	for _, tc := range []struct {
		name         string
		manage, read bool
		want         string
	}{
		{name: "manager who reads the subject", manage: true, read: true, want: "salary 9000"},
		{name: "manager who cannot read the subject", manage: true, read: false, want: ""},
		{name: "reader who does not manage", manage: false, read: true, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := failedJobView()
			withholdJobError(&v, tc.manage, tc.read)
			require.Equal(t, tc.want, v.Waiting.Pending.Job.Error)
			require.Equal(t, tc.want, v.Parallel[0].Waiting.Pending.Job.Error)
		})
	}
}

// TestNameSubjects_WithheldSubject pins that a run whose subject the
// reader may not read is named by its flow alone: a stored title can hold
// subject values.
func TestNameSubjects_WithheldSubject(t *testing.T) {
	c := &FlowControl{rows: blindRows{}}
	runs := []flowview.Run{
		{Flow: "onboarding", FlowLabel: "Onboarding", Subject: "PER-1", Title: "Onboarding for Ada"},
		{Flow: "onboarding", FlowLabel: "Onboarding", Title: "Onboarding for Ada"},
		{Flow: "signup", Subject: "PER-2", Title: "signup Bob"},
	}
	require.NoError(t, c.nameSubjects(context.Background(), runs))
	for i, want := range []string{"Onboarding", "Onboarding", "signup"} {
		require.Equal(t, want, runs[i].Title)
		require.Empty(t, runs[i].Subject)
		require.Empty(t, runs[i].SubjectTitle)
	}
}
