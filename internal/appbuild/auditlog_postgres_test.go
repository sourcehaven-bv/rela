//go:build postgres

package appbuild_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// refuseAuditAppends makes every later append to the audit log fail.
func refuseAuditAppends(t *testing.T, svc *appbuild.Services) {
	t.Helper()
	db := pgstore.HandleFor(svc.Store())
	require.NotNil(t, db)
	_, err := db.Exec(context.Background(), `
		CREATE FUNCTION refuse_audit() RETURNS trigger LANGUAGE plpgsql AS
			$$ BEGIN RAISE EXCEPTION 'audit refused'; END $$;
		CREATE TRIGGER refuse_audit BEFORE INSERT ON audit_log
			FOR EACH ROW EXECUTE FUNCTION refuse_audit();`)
	require.NoError(t, err)
}
