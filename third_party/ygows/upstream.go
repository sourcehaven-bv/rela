package websocket

// UpstreamModule and UpstreamVersion name the upstream release this fork was
// taken from: `provider/websocket` in that module, at that version.
//
// When go.mod moves `github.com/reearth/ygo` to another version, re-sync the
// fork with that release and then update UpstreamVersion. TestUpstreamVersion
// fails until both agree. See README.md, "Keeping the fork current".
const (
	UpstreamModule  = "github.com/reearth/ygo"
	UpstreamVersion = "v1.55.0"
)
