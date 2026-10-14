// Package access is the access service of the data-entry API: it decides
// what one HTTP request may read and write.
//
// It owns the decisions that depend on which ACL is configured ([Mode]) and
// the per-request lifecycle ([Opener]): one acl.Request is opened per HTTP
// request, after principal resolution, and every gate in that request uses
// it.
//
// The package grows in steps (TKT-WFS778). Later steps move the read gate,
// the gated reader, write authorization and affordance verdicts here, after
// which the data-entry package no longer imports the acl and affordances
// packages and the architecture lint enforces that.
package access
