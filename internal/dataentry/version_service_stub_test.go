package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// stubVersionService is a test-only store.VersionService whose methods all panic.
// A history-handler test embeds it and overrides only the one or two reader
// methods it exercises, so it satisfies the umbrella interface without spelling
// out every unused write/purge method. (Since the version service was extracted
// from the store — a store just stores — the handlers bind the narrow reader
// sub-interface, but App.versions is typed as the umbrella, so a fake assigned to
// it must satisfy the whole surface.)
type stubVersionService struct{}

func (stubVersionService) ListVersions(context.Context, string) ([]store.VersionMeta, error) {
	panic("stubVersionService.ListVersions not implemented")
}

func (stubVersionService) GetVersion(context.Context, string, int) (*store.VersionSnapshot, error) {
	panic("stubVersionService.GetVersion not implemented")
}

func (stubVersionService) WriteVersion(context.Context, store.VersionInput) error {
	panic("stubVersionService.WriteVersion not implemented")
}

func (stubVersionService) ListRelationVersions(
	context.Context, store.RelationHistoryQuery,
) ([]store.RelationVersionMeta, error) {
	panic("stubVersionService.ListRelationVersions not implemented")
}

func (stubVersionService) GetRelationVersion(
	context.Context, store.RelationHistoryQuery, int,
) (*store.RelationVersionSnapshot, error) {
	panic("stubVersionService.GetRelationVersion not implemented")
}

func (stubVersionService) ListRelationLifetimes(
	context.Context, string, string, string,
) ([]store.RelationLifetime, error) {
	panic("stubVersionService.ListRelationLifetimes not implemented")
}

func (stubVersionService) WriteRelationVersion(context.Context, store.RelationVersionInput) error {
	panic("stubVersionService.WriteRelationVersion not implemented")
}

func (stubVersionService) PurgeVersions(
	context.Context, store.VersionPurgeRequest,
) (*store.PurgeResult, error) {
	panic("stubVersionService.PurgeVersions not implemented")
}

func (stubVersionService) PurgeRelationVersions(
	context.Context, store.RelationVersionPurgeRequest,
) (*store.PurgeResult, error) {
	panic("stubVersionService.PurgeRelationVersions not implemented")
}

var _ store.VersionService = stubVersionService{}
