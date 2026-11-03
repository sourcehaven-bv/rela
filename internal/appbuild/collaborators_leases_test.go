package appbuild_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// A Services assembled from collaborators offers the lease service, as an
// assembled project's does, so a surface built over it serves the lease
// routes.
func TestNewFromCollaborators_OffersLeases(t *testing.T) {
	meta, err := metamodel.Parse([]byte(copyWiringMeta))
	if err != nil {
		t.Fatal(err)
	}
	svc := appbuildtest.New(meta, appbuildtest.WithStore(memstore.New()))
	if appbuild.Leases(svc) == nil {
		t.Error("a Services built from collaborators must offer the lease service")
	}
}
