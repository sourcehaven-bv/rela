package v1

// PresenceViewer is one person who has the entity open (TKT-MHXH87). Since
// is RFC 3339 UTC in whole seconds: when they opened it, kept across a
// reconnect within the grace period.
type PresenceViewer struct {
	User  string `json:"user"`
	Tool  string `json:"tool"`
	Since string `json:"since"`
}

// PresenceResponse is the `GET /{plural}/{id}/_presence` answer: the other
// viewers of the entity's face, oldest first, at most 20. More counts the
// viewers past that cap and is always present.
type PresenceResponse struct {
	Viewers []PresenceViewer `json:"viewers"`
	More    int              `json:"more"`
}
