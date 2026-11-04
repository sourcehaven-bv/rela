package v1

// PileSummary is one pile as `GET /_piles` lists it (TKT-K3RJLH): no items,
// and a count of the items the owner may read in the request's world.
type PileSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Icon    string `json:"icon"`
	Count   int    `json:"count"`
	Created string `json:"created"`
	Updated string `json:"updated"`
	// Key is the schema key of a declared pile; empty for a pile the user
	// made.
	Key string `json:"key,omitempty"`
	// Locked marks a declared pile: it cannot be renamed, re-iconed or
	// deleted.
	Locked bool `json:"locked,omitempty"`
	// ReadState marks a pile whose items are read or unread.
	ReadState bool `json:"read_state,omitempty"`
	// AutoRead marks a pile whose items become read when the owner opens
	// the entity.
	AutoRead bool `json:"auto_read,omitempty"`
	// Unread is the number of readable unread items.
	Unread int `json:"unread,omitempty"`
	// Addresses are the readable items of a declared pile, so a toggle can
	// tell whether an entity is on it without a second request.
	Addresses []string `json:"addresses,omitempty"`
	// UnreadAddresses are the readable unread items of a pile with read
	// state.
	UnreadAddresses []string `json:"unread_addresses,omitempty"`
}

// PileItem is one readable item on a pile.
type PileItem struct {
	// ID is the bare entity id.
	ID string `json:"id"`
	// Face is "" for the implicit face.
	Face string `json:"face"`
	// Address is `ID` or `ID@face`: what links, removal and actions name.
	Address string `json:"address"`
	Type    string `json:"type"`
	// Title is the display title of the redacted row.
	Title string `json:"title"`
	// Added is when the item was put on the pile (RFC 3339).
	Added string `json:"added,omitempty"`
	// Read reports that the owner has read the item; only on a pile with
	// read state.
	Read bool `json:"read,omitempty"`
	// Actor names the user whose write put the item here: the person's
	// title, or a login when users are not mapped to people. Empty when
	// unknown or not readable.
	Actor string `json:"actor,omitempty"`
	// Reason is the label of the declared reason; empty for none.
	Reason string `json:"reason,omitempty"`
}

// Pile is one pile with its readable items, newest first.
type Pile struct {
	PileSummary
	Items []PileItem `json:"items"`
}

// PileList is the `GET /_piles` response: the owner's piles, oldest first,
// and the icon names a pile may use.
type PileList struct {
	Piles []PileSummary `json:"piles"`
	Icons []string      `json:"icons"`
}

// PileCreateRequest is the `POST /_piles` body.
type PileCreateRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
	// Items are addresses (`ID` or `ID@face`) to put on the new pile.
	Items []string `json:"items,omitempty"`
}

// PileUpdateRequest is the `PATCH /_piles/{id}` body. A nil field is left
// unchanged.
type PileUpdateRequest struct {
	Name *string `json:"name,omitempty"`
	Icon *string `json:"icon,omitempty"`
}

// PileItemsRequest is the body of `POST /_piles/{id}/items` and of
// `POST /_piles/{id}/items/_remove`: addresses (`ID` or `ID@face`).
type PileItemsRequest struct {
	Items []string `json:"items"`
}

// PileAdded is the `POST /_piles/{id}/items` response: how many addresses
// were new on the pile. It never says what an add evicted.
type PileAdded struct {
	Added int `json:"added"`
}
