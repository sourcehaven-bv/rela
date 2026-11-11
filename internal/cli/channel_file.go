package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// channelFileOpener opens a file of a message. The wiring site passes the
// project's *realtime.FileOpener, which runs the channel's read check.
type channelFileOpener interface {
	OpenFile(ctx context.Context, addr realtime.RawAddress, seq int64, id string) (realtime.FileDownload, error)
}

// readPostFiles reads the --file values of a post: each is PROP=PATH, and
// the file keeps the base name of its path.
//
// Nil: returned when there are none.
func readPostFiles(values []string) ([]realtime.FileInput, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > realtime.MaxFilesPerMessage {
		return nil, fmt.Errorf("--file: at most %d files", realtime.MaxFilesPerMessage)
	}
	out := make([]realtime.FileInput, 0, len(values))
	total := 0
	for _, v := range values {
		prop, path, ok := strings.Cut(v, "=")
		if !ok || prop == "" || path == "" {
			return nil, fmt.Errorf("--file %q: want PROP=PATH", v)
		}
		data, err := readCapped(path, realtime.MaxFileBytes)
		if err != nil {
			return nil, fmt.Errorf("--file %s: %w", prop, err)
		}
		if total += len(data); total > realtime.MaxMessageFileBytes {
			return nil, fmt.Errorf("--file: over %d bytes together", realtime.MaxMessageFileBytes)
		}
		out = append(out, realtime.FileInput{Property: prop, Name: filepath.Base(path), Data: data})
	}
	return out, nil
}

// readCapped reads path, refusing one over limit bytes.
func readCapped(path string, limit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("over %d bytes", limit)
	}
	return data, nil
}

// ChannelFileCmd downloads one file of a message.
type ChannelFileCmd struct {
	channelAddr
	Seq  int64  `arg:"" help:"Seq of the message."`
	File string `arg:"" name:"file-id" help:"File id, from rela channel read."`
	To   string `name:"to" placeholder:"PATH" help:"Write the file to PATH, or to stdout with -. Without --to: stdout, unless a terminal."`
}

// Run writes the file.
func (c *ChannelFileCmd) Run(ctx context.Context, cs *channelServices) error {
	if _, err := cs.svc(); err != nil {
		return err
	}
	if c.Seq < 1 {
		return errors.New("seq: at least 1")
	}
	addr := c.raw()
	got, err := cs.Files.OpenFile(ctx, addr, c.Seq, c.File)
	if err != nil {
		return channelFailed(ctx, "read file", addr.Anchor, err)
	}
	defer got.Body.Close()
	switch c.To {
	case "-":
		_, err = io.Copy(os.Stdout, got.Body)
		return err
	case "":
		if isTerminal(os.Stdout) {
			return errors.New("refusing to write a file to a terminal; pass --to PATH, or --to - to write it anyway")
		}
		_, err = io.Copy(os.Stdout, got.Body)
		return err
	}
	return writeFileAtomic(c.To, got.Body)
}

// writeFileAtomic writes r to a temporary file next to path and renames
// it into place, so a failed download leaves no partial file at path.
func writeFileAtomic(path string, r io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(tmp, r)
	if err := errors.Join(copyErr, tmp.Close()); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}
