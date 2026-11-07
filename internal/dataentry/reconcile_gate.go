package dataentry

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// reconcileGate admits reconcile requests (RR-ER381BQ, RR-3RVT441). A
// request first takes a token from its principal's bucket, then a slot: one
// of a few per server, and at most a few per principal, so one peer cannot
// hold every slot. It is built once per handler and shared; a per-request
// gate would bound nothing. The App is rebuilt on a configuration reload,
// and with it the gate, so a reload gives every principal a full bucket.
//
// Refusals are immediate, never queued: a peer retries after Retry-After,
// which costs the server nothing while it waits.
type reconcileGate struct {
	cfg   reconcileGateConfig
	slots chan struct{}

	mu    sync.Mutex
	peers map[string]*reconcilePeer
}

// reconcilePeer is one principal's bucket and the slots it holds.
type reconcilePeer struct {
	limiter *rate.Limiter
	inUse   int
	seen    time.Time
}

// reconcileGateConfig sets the gate's limits. A zero field takes the
// default below.
type reconcileGateConfig struct {
	// Slots is the number of requests the server runs at once.
	Slots int
	// PerPrincipal is the number of those one principal may hold.
	PerPrincipal int
	// Rate and Burst are each principal's token bucket, in requests.
	Rate  rate.Limit
	Burst int
	// MaxPeers caps the principals tracked. A full table evicts idle
	// principals (see evictIdle); a new principal is refused with 503 only
	// when every tracked one holds a slot.
	MaxPeers int
	IdleTTL  time.Duration
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

const (
	defaultReconcileSlots        = 4
	defaultReconcilePerPrincipal = 2
	defaultReconcileRate         = rate.Limit(2)
	defaultReconcileBurst        = 20
	defaultReconcileMaxPeers     = 1024
	defaultReconcileIdleTTL      = 10 * time.Minute
)

func newReconcileGate(cfg reconcileGateConfig) *reconcileGate {
	if cfg.Slots <= 0 {
		cfg.Slots = defaultReconcileSlots
	}
	if cfg.PerPrincipal <= 0 {
		cfg.PerPrincipal = defaultReconcilePerPrincipal
	}
	if cfg.Rate <= 0 {
		cfg.Rate = defaultReconcileRate
	}
	if cfg.Burst <= 0 {
		cfg.Burst = defaultReconcileBurst
	}
	if cfg.MaxPeers <= 0 {
		cfg.MaxPeers = defaultReconcileMaxPeers
	}
	if cfg.IdleTTL <= 0 {
		cfg.IdleTTL = defaultReconcileIdleTTL
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &reconcileGate{cfg: cfg, slots: make(chan struct{}, cfg.Slots), peers: map[string]*reconcilePeer{}}
}

// acquire admits one request of user. On success it returns the release
// function, which the caller must call once; otherwise the refusal.
//
// A refused request costs its principal nothing: the token it reserved is
// handed back, so a peer that retries after Retry-After is not pushed
// further out by its own refusals.
func (g *reconcileGate) acquire(user string) (func(), *reconcileRefusal) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.cfg.Now()
	p := g.peers[user]
	if p == nil {
		if len(g.peers) >= g.cfg.MaxPeers {
			g.evictIdle(now)
		}
		if len(g.peers) >= g.cfg.MaxPeers {
			return nil, busyRefusal()
		}
		p = &reconcilePeer{limiter: rate.NewLimiter(g.cfg.Rate, g.cfg.Burst)}
		g.peers[user] = p
	}
	p.seen = now
	res := p.limiter.ReserveN(now, 1)
	if wait := res.DelayFrom(now); wait > 0 {
		res.CancelAt(now)
		return nil, &reconcileRefusal{
			status: http.StatusTooManyRequests, code: "too_many_requests",
			title: "Too many reconcile requests", retryAfter: wait,
		}
	}
	if p.inUse >= g.cfg.PerPrincipal {
		res.CancelAt(now)
		return nil, busyRefusal()
	}
	select {
	case g.slots <- struct{}{}:
	default:
		res.CancelAt(now)
		return nil, busyRefusal()
	}
	p.inUse++
	var once sync.Once
	return func() {
		once.Do(func() {
			<-g.slots
			g.mu.Lock()
			p.inUse--
			p.seen = g.cfg.Now()
			g.mu.Unlock()
		})
	}, nil
}

// evictIdle makes room in a full peer table. It drops every principal that
// holds no slot and has not been seen for IdleTTL; when that frees nothing,
// it drops the idle principal seen longest ago, so a burst of new
// principals cannot lock out the next one for IdleTTL. A dropped principal
// starts again with a full bucket, which is no more than a new principal
// gets. Only when every tracked principal holds a slot does nothing go,
// which the slot cap makes impossible at the default sizes.
func (g *reconcileGate) evictIdle(now time.Time) {
	var oldest string
	var oldestSeen time.Time
	for user, p := range g.peers {
		if p.inUse > 0 {
			continue
		}
		if now.Sub(p.seen) >= g.cfg.IdleTTL {
			delete(g.peers, user)
			continue
		}
		if oldest == "" || p.seen.Before(oldestSeen) {
			oldest, oldestSeen = user, p.seen
		}
	}
	if len(g.peers) >= g.cfg.MaxPeers && oldest != "" {
		delete(g.peers, oldest)
	}
}

func busyRefusal() *reconcileRefusal {
	return &reconcileRefusal{
		status: http.StatusServiceUnavailable, code: "reconcile_busy",
		title: "Too many reconcile requests are running", retryAfter: time.Second,
	}
}
