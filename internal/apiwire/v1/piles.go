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
