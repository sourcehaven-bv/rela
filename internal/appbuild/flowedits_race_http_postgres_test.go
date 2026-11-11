//go:build postgres

package appbuild_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// The upload's race test runs on postgres only. On sqlite the test's held
// write transaction also blocks the upload's blob write, which comes before
// the stamp, so the upload fails with SQLITE_BUSY before it reaches the
// recheck. The stamp's recheck is the same code on both tiers.
func TestOutsideEdits_RunStartedDuringAttachmentUploadFailsClosed(t *testing.T) {
	svc, router := raceDataEntry(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "scan.txt")
	require.NoError(t, err)
	_, err = fw.Write([]byte("scan"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := raceRequest(t, http.MethodPut, personsPath(t, svc)+"/PER-1/_attachments/scan",
		mw.FormDataContentType(), &body)
	requireRaceRefused(t, raceHTTP(t, svc, router, req, "entitymanager.Attachments.StampAttachments"), "scan")
}
