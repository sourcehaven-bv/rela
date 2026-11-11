package appbuild

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

func TestChannelFileChecker(t *testing.T) {
	key := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	tests := []struct {
		name     string
		accept   []string
		file     string
		data     []byte
		wantName string
		wantMIME string
		refused  string
	}{
		{name: "accepted", accept: []string{"image/png"}, file: "shot.png", data: png,
			wantName: "shot.png", wantMIME: "image/png"},
		{name: "control chars replaced", file: "a\x7fb\u0085c.txt", data: []byte("hello"),
			wantName: "a_b_c.txt", wantMIME: "text/plain"},
		{name: "invalid utf8 replaced", file: "a\xffb.txt", data: []byte("hello"),
			wantName: "a_b.txt", wantMIME: "text/plain"},
		{name: "outside accept", accept: []string{"image/png"}, file: "note.txt", data: []byte("hello"),
			refused: "not in the allowed list"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sink := audit.NewMemory()
			c := newChannelFileChecker(nil, nil, sink)
			got, err := c.CheckFile(context.Background(), realtime.FileCheck{
				Channel: key, Property: "shot", Policy: metamodel.PropertyDef{Accept: tc.accept},
				Name: tc.file, Data: tc.data,
			})
			if tc.refused != "" {
				require.ErrorIs(t, err, realtime.ErrFileRefused)
				require.Contains(t, err.Error(), tc.refused)
				recs := sink.Records()
				require.Len(t, recs, 1)
				require.Equal(t, audit.OpDeniedWrite, recs[0].Op)
				require.Equal(t, "TKT-1", recs[0].Subject.ID)
				require.True(t, strings.HasSuffix(recs[0].Summary, "(op=channel-file-write)"), recs[0].Summary)
				require.Contains(t, recs[0].Summary, "TKT-1/room")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantName, got.Name)
			require.Equal(t, tc.wantMIME, got.MIME)
			require.Empty(t, sink.Records())
		})
	}
	// A nil sink discards the refusal record.
	_, err := newChannelFileChecker(nil, nil, nil).CheckFile(context.Background(), realtime.FileCheck{
		Channel: key, Property: "shot", Policy: metamodel.PropertyDef{Accept: []string{"image/png"}},
		Name: "x.txt", Data: []byte("x"),
	})
	require.ErrorIs(t, err, realtime.ErrFileRefused)
}

type stubRunner struct{}

func (stubRunner) Scan(context.Context, []string, []byte) error { return nil }

func (stubRunner) Transform(
	_ context.Context, _ []string, _ attachment.ProcessContext, d []byte,
) (out []byte, name string, err error) {
	return d, "", nil
}

func TestSetChannelFileRunner(t *testing.T) {
	require.Error(t, SetChannelFileRunner(&Services{}, stubRunner{}), "unassembled services are refused")
	runners := &channelFileRunners{}
	s := &Services{base: &SharedBase{channelFiles: runners}}
	require.Error(t, SetChannelFileRunner(s, nil), "a nil runner is refused")
	require.Nil(t, runners.runner())
	require.NoError(t, SetChannelFileRunner(s, stubRunner{}))
	require.Equal(t, stubRunner{}, runners.runner())

	var none *channelFileRunners
	require.Nil(t, none.runner(), "a nil holder has no runner")
}
