package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// withheldHost is the host config of [WithWithheldHostConfig]: it serves
// mail.yaml from inner and withholds ai.yaml and the secrets with err,
// which wraps [lua.ErrHostConfigWithheld].
//
// Mail does not read through it: [mailHost] hands the mail outbox and
// mail.send the inner config, so the SMTP password in secrets.yaml still
// reaches the transport and nothing else.
type withheldHost struct {
	inner lua.HostConfig
	err   error
}

// File serves mail.yaml and withholds every other file.
func (h withheldHost) File(name string) ([]byte, error) {
	if name == hostconfig.MailFile {
		return h.inner.File(name)
	}
	return nil, h.err
}

// Secrets withholds the secrets.
func (h withheldHost) Secrets(string) (map[string]string, error) { return nil, h.err }

// Path is the inner config's .rela directory.
func (h withheldHost) Path() string { return h.inner.Path() }

// mailHost is the host config mail reads: the inner config of a
// [withheldHost], else host itself.
func mailHost(host lua.HostConfig) lua.HostConfig {
	if w, ok := host.(withheldHost); ok {
		return w.inner
	}
	return host
}
