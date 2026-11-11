package dataentry

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

const (
	// messagePart is the multipart part that carries the post's JSON body.
	messagePart = "message"
	// filePartPrefix starts the name of a file part: `file:<property>`.
	filePartPrefix = "file:"
	// multipartHeadroom is room for the part headers and boundaries of a
	// multipart post, on top of its JSON part and file bytes.
	multipartHeadroom = 64 << 10
	// multipartBodyLimit caps a multipart post body.
	multipartBodyLimit = channelBodyLimit + realtimelimits.MaxMessageFileBytes + multipartHeadroom
)

// channelFileOpener serves the file download route and checks a post
// with files before its files are read.
type channelFileOpener interface {
	OpenFile(ctx context.Context, addr realtime.RawAddress, seq int64, id string) (realtime.FileDownload, error)
	CheckUpload(ctx context.Context, addr realtime.RawAddress, msgType string) error
}

// uploadSlots bounds concurrent uploads. *attachment.Limiter satisfies it;
// channel posts share the entity uploads' slots.
type uploadSlots interface {
	TryAcquire() (release func(), ok bool)
}

func isMultipart(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "multipart/form-data"
}

// postMultipart posts a message with files. The body is
// `multipart/form-data`: first a `message` part holding the JSON body of
// a plain post, then one `file:<property>` part per file, at most
// [channels.MaxFilesPerMessage]. Any other part, a missing or late
// `message` part, or a file part without a file name is a 400. It reads
// the small message part, checks that the caller may post the type with
// files, and only then takes an upload slot and reads at most
// [realtimelimits.MaxMessageFileBytes] of file bytes, so a caller who may
// not post holds no slot. The service checks the per-property limits and
// the content.
func (h *channelsHandler) postMultipart(w http.ResponseWriter, r *http.Request, rt channelRoute) {
	r.Body = http.MaxBytesReader(w, r.Body, multipartBodyLimit)
	mr, err := r.MultipartReader()
	if err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid multipart body", "")
		return
	}
	var req v1.PostMessageRequest
	if !readMessagePart(w, r, mr, &req) {
		return
	}
	if err := h.files.CheckUpload(r.Context(), rt.addr, req.Type); err != nil {
		writeChannelError(w, r, err)
		return
	}
	release, ok := h.uploads.TryAcquire()
	if !ok {
		writeAttachmentBusy(w, r, attachment.ErrBusy)
		return
	}
	defer release()
	files, ok := readFileParts(w, r, mr)
	if !ok {
		return
	}
	h.postMessage(w, r, rt, req, files)
}

// readMessagePart reads the first part, which must be the `message` JSON.
func readMessagePart(w http.ResponseWriter, r *http.Request, mr *multipart.Reader, dst *v1.PostMessageRequest) bool {
	part, err := mr.NextPart()
	if err != nil {
		writeMultipartError(w, r, err, "the body needs a message part")
		return false
	}
	defer part.Close()
	if part.FormName() != messagePart || part.FileName() != "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid multipart body",
			"the first part must be the message part")
		return false
	}
	data, err := io.ReadAll(io.LimitReader(part, channelBodyLimit+1))
	if err != nil {
		writeMultipartError(w, r, err, "")
		return false
	}
	if len(data) > channelBodyLimit {
		writeV1Error(w, r, http.StatusRequestEntityTooLarge, realtime.TypeTooLarge, "Message part too large", "")
		return false
	}
	return decodeChannelJSON(w, r, data, dst)
}

// readFileParts reads the `file:<property>` parts after the message part.
func readFileParts(w http.ResponseWriter, r *http.Request, mr *multipart.Reader) ([]realtime.FileInput, bool) {
	var files []realtime.FileInput
	total := 0
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return files, true
		}
		if err != nil {
			writeMultipartError(w, r, err, "")
			return nil, false
		}
		f, ok := readFilePart(w, r, part, &total, len(files))
		part.Close()
		if !ok {
			return nil, false
		}
		files = append(files, f)
	}
}

func readFilePart(
	w http.ResponseWriter, r *http.Request, part *multipart.Part, total *int, have int,
) (realtime.FileInput, bool) {
	prop, isFile := strings.CutPrefix(part.FormName(), filePartPrefix)
	if !isFile || prop == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid multipart body",
			"unknown part "+strconv.Quote(part.FormName()))
		return realtime.FileInput{}, false
	}
	if part.FileName() == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid multipart body",
			"file part "+strconv.Quote(part.FormName())+" needs a file name")
		return realtime.FileInput{}, false
	}
	if have >= channels.MaxFilesPerMessage {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Too many files",
			"a message holds at most "+strconv.Itoa(channels.MaxFilesPerMessage)+" files")
		return realtime.FileInput{}, false
	}
	left := realtimelimits.MaxMessageFileBytes - *total
	data, err := io.ReadAll(io.LimitReader(part, int64(left)+1))
	if err != nil {
		writeMultipartError(w, r, err, "")
		return realtime.FileInput{}, false
	}
	if len(data) > left {
		writeV1Error(w, r, http.StatusRequestEntityTooLarge, realtime.TypeTooLarge, "Files too large",
			"the files of a message are over "+strconv.Itoa(realtimelimits.MaxMessageFileBytes)+" bytes together")
		return realtime.FileInput{}, false
	}
	*total += len(data)
	return realtime.FileInput{Property: prop, Name: part.FileName(), Data: data}, true
}

// writeMultipartError answers a failed multipart read: a 413 when the body
// is over its cap, otherwise a 400 with detail.
func writeMultipartError(w http.ResponseWriter, r *http.Request, err error, detail string) {
	if _, tooBig := errors.AsType[*http.MaxBytesError](err); tooBig {
		writeV1Error(w, r, http.StatusRequestEntityTooLarge, realtime.TypeTooLarge, "Request body too large", "")
		return
	}
	writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid multipart body", detail)
}

// downloadFile serves one file of a message as a hardened download: it is
// never rendered inline, whatever its type.
func (h *channelsHandler) downloadFile(w http.ResponseWriter, r *http.Request, rt channelRoute) {
	got, err := h.files.OpenFile(r.Context(), rt.addr, rt.seq, rt.file)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	defer got.Body.Close()
	setHardenedDownloadHeaders(w.Header(), got.File.MIME, got.File.Name)
	w.Header().Set("Content-Length", strconv.FormatInt(got.File.Size, 10))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	// The status is sent; a client gone mid-copy has nothing to hear.
	_, _ = io.Copy(w, got.Body)
}

// AttachmentRunner returns the command runner the app built for upload
// scans and transforms, after its check that the sandbox does not expose
// the project. The host hands it to the channel service.
//
// Nil: returned when the host cannot run commands.
func AttachmentRunner(a *App) attachment.CommandRunner {
	return a.attachmentRunner
}
