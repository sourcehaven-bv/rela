package v1

// EntityLease is the `_lease` an entity GET and a list row carry while
// someone holds an advisory lease on the entity (TKT-4YPG4Q). It has no
// expiry and no lease id: a plain renew must not change the response, and
// the lease id is the holder's secret. Status is free text the holder wrote.
type EntityLease struct {
	User   string `json:"user"`
	Tool   string `json:"tool"`
	Label  string `json:"label,omitempty"`
	Since  string `json:"since"`
	Status string `json:"status,omitempty"`
}

// LeaseClaimRequest is the `POST /{plural}/{id}/_lease` body. Every field is
// optional; an empty body claims for the default duration.
type LeaseClaimRequest struct {
	// TTLSeconds is the lease duration, 30 to 3600; 0 means 900.
	TTLSeconds int    `json:"ttl_seconds,omitempty"`
	Label      string `json:"label,omitempty"`
	Status     string `json:"status,omitempty"`
}

// LeaseClaimed is the 201 answer to a claim. LeaseID renews and releases
// the lease, and is returned here only.
type LeaseClaimed struct {
	LeaseID   string `json:"lease_id"`
	Token     uint64 `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// LeaseRenewRequest is the `POST /_leases/renew` body. A nil Status keeps
// the status; an empty one clears it.
type LeaseRenewRequest struct {
	LeaseID    string  `json:"lease_id"`
	TTLSeconds int     `json:"ttl_seconds,omitempty"`
	Status     *string `json:"status,omitempty"`
}

// LeaseRenewed is the 200 answer to a renew.
type LeaseRenewed struct {
	Token     uint64 `json:"token"`
	ExpiresAt string `json:"expires_at"`
	Status    string `json:"status,omitempty"`
}

// LeaseReleaseRequest is the `POST /_leases/release` body.
type LeaseReleaseRequest struct {
	LeaseID string `json:"lease_id"`
}

// OwnLease is one of the caller's leases in `GET /_leases`. It has no lease
// id.
type OwnLease struct {
	ID        string `json:"id"`
	Since     string `json:"since"`
	ExpiresAt string `json:"expires_at"`
	Status    string `json:"status,omitempty"`
	Label     string `json:"label,omitempty"`
}

// OwnLeaseList is the `GET /_leases` answer: the caller's live leases on
// entities it may read, oldest first.
type OwnLeaseList struct {
	Data []OwnLease `json:"data"`
}

// LeaseHolder names the holder in a `lease_held` refusal.
type LeaseHolder struct {
	User  string `json:"user"`
	Tool  string `json:"tool"`
	Label string `json:"label,omitempty"`
}

// LeaseHeld is the 409 `lease_held` problem: the problem fields plus the
// live lease that refused the claim.
type LeaseHeld struct {
	Error
	Holder    LeaseHolder `json:"holder"`
	Since     string      `json:"since"`
	ExpiresAt string      `json:"expires_at"`
	Status    string      `json:"status,omitempty"`
}
