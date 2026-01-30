package dataentry

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SyncState represents the current git synchronization state.
type SyncState string

const (
	// SyncDisabled means git is not available or no remote is configured.
	SyncDisabled SyncState = "disabled"
	// SyncClean means the local branch matches the remote.
	SyncClean SyncState = "clean"
	// SyncAhead means there are unpushed local commits.
	SyncAhead SyncState = "ahead"
	// SyncSyncing means a fetch/push operation is in progress.
	SyncSyncing SyncState = "syncing"
	// SyncError means the last sync operation failed.
	SyncError SyncState = "error"
	// SyncConflict means a rebase conflict was detected.
	SyncConflict SyncState = "conflict"
	// SyncOffline means the remote is unreachable (network issue).
	SyncOffline SyncState = "offline"
)

// SyncStatus is the full sync state returned by the API and used in templates.
type SyncStatus struct {
	State     SyncState `json:"state"`
	Branch    string    `json:"branch"`
	Unpushed  int       `json:"unpushed"`
	Behind    int       `json:"behind"`
	Enabled   bool      `json:"enabled"`
	Protected bool      `json:"protected"`
	Message   string    `json:"message"`
	LastSync  string    `json:"last_sync,omitempty"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

// BranchList contains local and remote branch information.
type BranchList struct {
	Current string   `json:"current"`
	Local   []string `json:"local"`
	Remote  []string `json:"remote"`
}

// SyncOptions configures the SyncManager.
type SyncOptions struct {
	ProtectedBranches []string // branch name patterns (supports * globs)
	OnPull            func()   // called after fast-forward pull for graph rebuild
}

const (
	// commitDebounce is the delay after the last CommitAsync call before the commit runs.
	commitDebounce = 2 * time.Second
	// pushDebounce is the delay after the last commit before auto-push.
	pushDebounce = 5 * time.Second
	// fetchInterval is the periodic fetch interval when idle.
	fetchInterval = 30 * time.Second
	// maxBackoff is the maximum retry delay for transient errors.
	maxBackoff = 5 * time.Minute
)

// workKind discriminates the type of work item in the queue.
type workKind int

const (
	workCommit       workKind = iota // debounced fire-and-forget commit
	workSwitchBranch                 // synchronous branch switch
	workCreateBranch                 // synchronous branch creation
	workSync                         // fetch + squash + rebase + push
	workPush                         // force immediate push (user-triggered)
	workMoveToBranch                 // create + push + switch to new branch
)

// workItem is a unit of work for the git work queue.
type workItem struct {
	kind    workKind
	message string // commit message or branch name
	result  chan<- error
}

// SyncManager manages git operations for a rela project.
type SyncManager struct {
	mu       sync.RWMutex
	state    SyncState
	branch   string
	backend  GitBackend
	unpushed int
	behind   int
	enabled  bool
	// protected indicates the current branch is protected (e.g. main/master).
	protected bool
	message   string
	lastSync  time.Time
	lastError string

	protectedPatterns []string
	protectedCache    map[string]bool

	onPull func()

	// conflicts holds the active conflict set (set during rebase conflict).
	conflicts *ConflictSet

	// Unified work queue for all git-mutating operations
	workCh chan workItem
	done   chan struct{}

	// Retry backoff for transient/network errors.
	consecutiveFailures int

	// SSE subscribers for real-time sync status updates.
	subMu sync.Mutex
	subID int
	subs  map[int]chan SyncStatus
}

// NewSyncManager creates a SyncManager with the given GitBackend.
// If backend is nil, the manager starts in disabled state.
// The backend should be created via NewGitBackend (which checks for nested
// projects, git availability, etc.).
func NewSyncManager(backend GitBackend, opts SyncOptions) *SyncManager {
	s := &SyncManager{
		state:             SyncDisabled,
		backend:           backend,
		protectedPatterns: opts.ProtectedBranches,
		protectedCache:    make(map[string]bool),
		onPull:            opts.OnPull,
		subs:              make(map[int]chan SyncStatus),
	}

	if backend == nil {
		s.message = "Git not configured"
		return s
	}

	// Check for remote
	if !backend.HasRemote() {
		s.message = "No remote configured"
		log.Printf("Sync: no git remote in %s", backend.RepoRoot())
		return s
	}

	s.enabled = true
	s.workCh = make(chan workItem, 64)
	s.done = make(chan struct{})
	s.refreshState()
	go s.workLoop()
	log.Printf("Sync: enabled on branch %q in %s", s.branch, backend.RepoRoot())
	return s
}

// State returns the current sync state (thread-safe).
func (s *SyncManager) State() SyncState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Branch returns the current branch name (thread-safe).
func (s *SyncManager) Branch() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.branch
}

// Status returns the full sync status for API responses and templates.
func (s *SyncManager) Status() SyncStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := SyncStatus{
		State:     s.state,
		Branch:    s.branch,
		Unpushed:  s.unpushed,
		Behind:    s.behind,
		Enabled:   s.enabled,
		Protected: s.protected,
		Message:   s.message,
		ErrorMsg:  s.lastError,
	}
	if !s.lastSync.IsZero() {
		status.LastSync = s.lastSync.Format(time.RFC3339)
	}
	return status
}

// Commit stages all changes and creates a git commit with the given message.
// It sends the work to the queue and blocks until it completes.
// It is a no-op if git is not enabled.
func (s *SyncManager) Commit(message string) error {
	if !s.enabled {
		return nil
	}
	result := make(chan error, 1)
	s.workCh <- workItem{kind: workCommit, message: message, result: result}
	return <-result
}

// commitNow performs the actual git add + commit. Must only be called from workLoop.
func (s *SyncManager) commitNow(message string) error {
	// Stage all changes
	if err := s.backend.StageAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Check if there are staged changes
	clean, err := s.backend.IsClean()
	if err != nil {
		return fmt.Errorf("checking status: %w", err)
	}
	if clean {
		return nil // nothing to commit
	}

	// Commit
	if err := s.backend.Commit(message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	s.updateState()
	return nil
}

// CommitAsync queues a commit message to be committed asynchronously.
// It returns immediately. Multiple rapid calls are debounced: only one commit
// runs after the debounce period, using the last message received.
func (s *SyncManager) CommitAsync(message string) {
	if !s.enabled {
		return
	}
	select {
	case s.workCh <- workItem{kind: workCommit, message: message}:
	default:
		log.Printf("Sync: work queue full, dropping commit: %s", message)
	}
}

// Push triggers an immediate sync (fetch + squash + push). Blocks until complete.
func (s *SyncManager) Push() error {
	if !s.enabled {
		return nil
	}
	result := make(chan error, 1)
	s.workCh <- workItem{kind: workPush, result: result}
	return <-result
}

// Pull triggers an immediate sync (fetch + fast-forward/rebase). Blocks until complete.
func (s *SyncManager) Pull() error {
	if !s.enabled {
		return nil
	}
	result := make(chan error, 1)
	s.workCh <- workItem{kind: workSync, result: result}
	return <-result
}

// MoveToBranch creates a new branch from current HEAD, pushing with tracking,
// then switches to it. This is used when the user has unpushed commits on a
// protected branch and wants to move them to a working branch.
func (s *SyncManager) MoveToBranch(name string) error {
	if !s.enabled {
		return fmt.Errorf("git not enabled")
	}
	result := make(chan error, 1)
	s.workCh <- workItem{kind: workMoveToBranch, message: name, result: result}
	return <-result
}

// moveToBranchNow creates a new branch from HEAD and pushes with tracking.
// Must only be called from workLoop.
func (s *SyncManager) moveToBranchNow(name string) error {
	// Create new branch from current HEAD
	if err := s.backend.CheckoutNewBranch(name); err != nil {
		return fmt.Errorf("creating branch %q: %w", name, err)
	}

	// Push with upstream tracking
	if err := s.backend.PushNewBranch(name); err != nil {
		// Revert to original branch on push failure
		origBranch := s.readBranch()
		_ = s.backend.Checkout(origBranch)
		_ = s.backend.DeleteBranch(name)
		return fmt.Errorf("pushing branch %q: %w", name, err)
	}

	s.updateState()
	return nil
}

// syncNow performs the full sync sequence: fetch, squash, rebase, push.
// Must only be called from workLoop.
func (s *SyncManager) syncNow() error {
	s.setState(SyncSyncing, "Syncing...")

	// Fetch
	if err := s.backend.Fetch(); err != nil {
		errMsg := err.Error()
		if isNetworkError(errMsg) {
			s.setOffline()
			return fmt.Errorf("git fetch (offline): %w", err)
		}
		s.setError("Fetch failed")
		return fmt.Errorf("git fetch: %w", err)
	}

	// Fetch succeeded — clear any offline/failure tracking
	s.mu.Lock()
	s.consecutiveFailures = 0
	s.mu.Unlock()

	branch := s.readBranch()
	upstream := "origin/" + branch

	// Count ahead/behind
	ahead := s.countRevs(upstream + "..HEAD")
	behind := s.countRevs("HEAD.." + upstream)

	switch {
	case ahead > 0 && behind == 0:
		// Ahead only: squash + push
		if err := s.squashCommits(branch); err != nil {
			s.setError("Squash failed")
			return err
		}
		if err := s.pushNow(); err != nil {
			return err
		}

	case ahead > 0 && behind > 0:
		// Diverged: squash + rebase + push
		if err := s.squashCommits(branch); err != nil {
			s.setError("Squash failed")
			return err
		}
		if err := s.backend.Rebase(upstream); err != nil {
			// Rebase failed — abort and enter conflict state
			_ = s.backend.AbortRebase()
			s.setConflict()
			return fmt.Errorf("rebase conflict on %s", branch)
		}
		if err := s.pushNow(); err != nil {
			return err
		}

	case ahead == 0 && behind > 0:
		// Behind only: fast-forward
		if err := s.backend.FastForwardMerge(upstream); err != nil {
			s.setError("Fast-forward failed")
			return fmt.Errorf("git merge --ff-only: %w", err)
		}
		s.mu.RLock()
		onPull := s.onPull
		s.mu.RUnlock()
		if onPull != nil {
			onPull()
		}

	default:
		// Already clean
	}

	s.mu.Lock()
	s.lastSync = time.Now()
	s.lastError = ""
	s.mu.Unlock()

	s.updateState()
	return nil
}

// squashCommits squashes all unpushed commits into a single commit.
// Must only be called from workLoop.
func (s *SyncManager) squashCommits(branch string) error {
	upstream := "origin/" + branch

	// Collect commit messages from unpushed commits
	messages, err := s.backend.LogMessages(upstream + "..HEAD")
	if err != nil {
		return fmt.Errorf("collecting commit messages: %w", err)
	}

	if len(messages) <= 1 {
		return nil // single commit or no commits, nothing to squash
	}

	// Build squash message
	var sb strings.Builder
	fmt.Fprintf(&sb, "rela: sync (%d changes)\n\n", len(messages))
	for _, m := range messages {
		// Strip the "rela: " prefix for the summary lines
		summary := strings.TrimPrefix(m, "rela: ")
		fmt.Fprintf(&sb, "- %s\n", summary)
	}

	// Soft reset to upstream
	if err := s.backend.SoftReset(upstream); err != nil {
		return fmt.Errorf("git reset --soft: %w", err)
	}

	// Create single squashed commit
	if err := s.backend.Commit(sb.String()); err != nil {
		return fmt.Errorf("squash commit: %w", err)
	}

	return nil
}

// pushNow pushes to origin. Parses stderr for protected branch detection.
// Must only be called from workLoop.
func (s *SyncManager) pushNow() error {
	branch := s.readBranch()

	output, err := s.backend.Push(branch)
	if err != nil {
		// Check for branch protection
		if isProtectedPushError(output) {
			s.mu.Lock()
			s.protectedCache[branch] = true
			s.protected = true
			s.mu.Unlock()
			s.setError("Protected branch — create a working branch")
			return fmt.Errorf("push rejected: protected branch %q", branch)
		}
		s.setError("Push failed")
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// isProtectedPushError checks git push stderr for branch protection messages.
func isProtectedPushError(stderr string) bool {
	lower := strings.ToLower(stderr)
	protectionPatterns := []string{
		"gh006",                // GitHub branch protection
		"protected branch",     // Generic
		"remote rejected",      // GitLab/generic
		"pre-receive hook",     // Server-side hooks
		"required status",      // GitHub status checks
		"changes must be made", // GitHub
	}
	for _, pattern := range protectionPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// isNetworkError checks whether a git command error is caused by network issues.
func isNetworkError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	patterns := []string{
		"could not resolve host",
		"unable to access",
		"connection refused",
		"connection timed out",
		"network is unreachable",
		"no route to host",
		"name or service not known",
		"couldn't connect to server",
		"the requested url returned error: 503",
		"the requested url returned error: 502",
		"ssl",
		"couldn't resolve host",
		"failed to connect",
		"timed out",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// backoffDuration returns an exponential backoff duration based on the failure count.
// The base interval is fetchInterval (30s), doubling each time, capped at maxBackoff (5m).
func backoffDuration(failures int) time.Duration {
	if failures <= 0 {
		return fetchInterval
	}
	d := fetchInterval
	for i := 0; i < failures && d < maxBackoff; i++ {
		d *= 2
	}
	if d > maxBackoff {
		d = maxBackoff
	}
	return d
}

// isProtected checks if the current branch is protected by config patterns or runtime cache.
func (s *SyncManager) isProtected() bool {
	s.mu.RLock()
	branch := s.branch
	s.mu.RUnlock()
	return s.isBranchProtected(branch)
}

// isBranchProtected checks if a specific branch is protected by config patterns or runtime cache.
func (s *SyncManager) isBranchProtected(branch string) bool {
	if branch == "" {
		return false
	}

	// Check runtime cache first
	s.mu.RLock()
	if cached, ok := s.protectedCache[branch]; ok {
		s.mu.RUnlock()
		return cached
	}
	s.mu.RUnlock()

	// Check config patterns
	for _, pattern := range s.protectedPatterns {
		if matched, _ := filepath.Match(pattern, branch); matched {
			s.mu.Lock()
			s.protectedCache[branch] = true
			s.mu.Unlock()
			return true
		}
	}

	return false
}

// workLoop runs in a goroutine and processes all git-mutating operations sequentially.
// Commits are debounced; branch operations flush pending commits and execute immediately.
// coverage-ignore: goroutine loop
func (s *SyncManager) workLoop() {
	defer close(s.done)

	var pendingCommit string
	commitTimer := time.NewTimer(0)
	commitTimer.Stop()

	pushTimer := time.NewTimer(0)
	pushTimer.Stop()

	fetchTicker := time.NewTicker(fetchInterval)
	defer fetchTicker.Stop()

	flushPending := func() {
		if pendingCommit != "" {
			if err := s.commitNow(pendingCommit); err != nil {
				log.Printf("Auto-commit failed: %v", err)
			}
			pendingCommit = ""
		}
	}

	triggerSync := func() {
		if s.isProtected() {
			// On protected branches, still fetch to show behind count
			if err := s.backend.Fetch(); err != nil {
				if isNetworkError(err.Error()) {
					s.setOffline()
				} else {
					log.Printf("Sync: fetch failed: %v", err)
				}
			} else {
				// Fetch succeeded, clear offline state
				s.mu.Lock()
				s.consecutiveFailures = 0
				s.mu.Unlock()
			}
			s.updateState()
			return
		}
		if err := s.syncNow(); err != nil {
			log.Printf("Sync: %v", err)
		}
	}

	resetFetchTicker := func() {
		fetchTicker.Stop()
		s.mu.RLock()
		failures := s.consecutiveFailures
		s.mu.RUnlock()
		if failures > 0 {
			// Use backoff duration; create a new ticker with the backoff interval
			d := backoffDuration(failures)
			fetchTicker = time.NewTicker(d)
			log.Printf("Sync: backoff retry in %v (failures=%d)", d, failures)
		} else {
			fetchTicker = time.NewTicker(fetchInterval)
		}
	}

	for {
		select {
		case item, ok := <-s.workCh:
			if !ok {
				// Channel closed — flush any pending commit
				flushPending()
				return
			}

			switch item.kind {
			case workCommit:
				if item.result != nil {
					// Synchronous commit: flush pending first, then run this one
					flushPending()
					commitTimer.Stop()
					err := s.commitNow(item.message)
					item.result <- err
					if err == nil && !s.isProtected() {
						pushTimer.Reset(pushDebounce)
					}
				} else {
					// Async commit: debounce
					pendingCommit = item.message
					commitTimer.Reset(commitDebounce)
				}

			case workSwitchBranch:
				flushPending()
				commitTimer.Stop()
				pushTimer.Stop()
				item.result <- s.switchBranchNow(item.message)

			case workCreateBranch:
				flushPending()
				commitTimer.Stop()
				pushTimer.Stop()
				item.result <- s.createBranchNow(item.message)

			case workSync:
				flushPending()
				commitTimer.Stop()
				pushTimer.Stop()
				if item.result != nil {
					item.result <- s.syncNow()
				} else {
					triggerSync()
				}
				resetFetchTicker()

			case workPush:
				flushPending()
				commitTimer.Stop()
				pushTimer.Stop()
				item.result <- s.syncNow()
				resetFetchTicker()

			case workMoveToBranch:
				flushPending()
				commitTimer.Stop()
				pushTimer.Stop()
				item.result <- s.moveToBranchNow(item.message)
			}

		case <-commitTimer.C:
			flushPending()
			// After flushing a debounced commit, schedule push
			if !s.isProtected() {
				pushTimer.Reset(pushDebounce)
			}

		case <-pushTimer.C:
			triggerSync()
			resetFetchTicker()

		case <-fetchTicker.C:
			// Periodic fetch when idle (backoff-aware)
			triggerSync()
			resetFetchTicker()
		}
	}
}

// Close shuts down the work goroutine, flushing any pending commit.
func (s *SyncManager) Close() {
	if s.workCh != nil {
		close(s.workCh)
		<-s.done // wait for goroutine to finish
	}
}

// Branches returns the list of local and remote branches.
func (s *SyncManager) Branches() (BranchList, error) {
	if !s.enabled {
		return BranchList{}, nil
	}

	s.mu.RLock()
	currentBranch := s.branch
	s.mu.RUnlock()

	bl := BranchList{
		Current: currentBranch,
	}

	local, remote, err := s.backend.ListBranches()
	if err != nil {
		return bl, err
	}
	bl.Local = local
	sort.Strings(bl.Local)

	// Filter remote branches: strip "origin/" prefix, exclude those already local
	localSet := make(map[string]bool, len(local))
	for _, lb := range local {
		localSet[lb] = true
	}
	for _, rb := range remote {
		if strings.HasSuffix(rb, "/HEAD") {
			continue
		}
		shortName := rb
		if strings.Contains(rb, "/") {
			shortName = rb[strings.Index(rb, "/")+1:]
		}
		if !localSet[shortName] {
			bl.Remote = append(bl.Remote, shortName)
		}
	}
	sort.Strings(bl.Remote)

	return bl, nil
}

// SwitchBranch switches to an existing branch via the work queue.
func (s *SyncManager) SwitchBranch(name string) error {
	if !s.enabled {
		return fmt.Errorf("git not enabled")
	}
	result := make(chan error, 1)
	s.workCh <- workItem{kind: workSwitchBranch, message: name, result: result}
	return <-result
}

// switchBranchNow performs the actual branch switch. Must only be called from workLoop.
func (s *SyncManager) switchBranchNow(name string) error {
	// Try local checkout first
	if err := s.backend.Checkout(name); err != nil {
		// Try checking out a remote tracking branch
		if err2 := s.backend.CheckoutNewBranchFrom(name, "origin/"+name); err2 != nil {
			return fmt.Errorf("switching to branch %q: %w", name, err)
		}
	}

	s.updateState()
	return nil
}

// CreateBranch creates a new branch from the current HEAD and switches to it via the work queue.
func (s *SyncManager) CreateBranch(name string) error {
	if !s.enabled {
		return fmt.Errorf("git not enabled")
	}
	result := make(chan error, 1)
	s.workCh <- workItem{kind: workCreateBranch, message: name, result: result}
	return <-result
}

// createBranchNow performs the actual branch creation. Must only be called from workLoop.
func (s *SyncManager) createBranchNow(name string) error {
	if err := s.backend.CheckoutNewBranch(name); err != nil {
		return fmt.Errorf("creating branch %q: %w", name, err)
	}

	s.updateState()
	return nil
}

// RepoRoot returns the git repository root directory.
func (s *SyncManager) RepoRoot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.backend == nil {
		return ""
	}
	return s.backend.RepoRoot()
}

// refreshState updates branch, unpushed count, and state during initialization.
func (s *SyncManager) refreshState() {
	if !s.enabled {
		return
	}
	s.updateState()
}

// updateState reads the current git state and updates the SyncManager fields under a write lock.
// Called from workLoop (which does not hold the lock) and during init.
func (s *SyncManager) updateState() {
	branchName := s.readBranch()
	upstream := "origin/" + branchName
	unpushed := s.countRevs(upstream + "..HEAD")
	behind := s.countRevs("HEAD.." + upstream)
	isProtected := s.isBranchProtected(branchName)

	s.mu.Lock()
	s.branch = branchName
	s.unpushed = unpushed
	s.behind = behind
	s.protected = isProtected

	switch {
	case s.state == SyncConflict:
		// Don't override conflict state
	case s.state == SyncError:
		// Keep error state until next successful sync
	case s.state == SyncOffline && s.consecutiveFailures > 0:
		// Keep offline state until fetch succeeds (clears consecutiveFailures)
	case isProtected && unpushed > 0:
		s.state = SyncAhead
		s.message = fmt.Sprintf("Protected · %d local commit(s)", unpushed)
	case unpushed > 0:
		s.state = SyncAhead
		s.message = fmt.Sprintf("%d unpushed commit(s)", unpushed)
	case behind > 0:
		s.state = SyncAhead
		s.message = fmt.Sprintf("%d behind remote", behind)
	default:
		s.state = SyncClean
		s.message = "Synced"
		s.lastError = ""
	}
	s.mu.Unlock()
	s.notifySubscribers()
}

// setState sets the sync state and message under a write lock.
func (s *SyncManager) setState(state SyncState, message string) {
	s.mu.Lock()
	s.state = state
	s.message = message
	s.mu.Unlock()
	s.notifySubscribers()
}

// setError transitions to error state with a message.
func (s *SyncManager) setError(msg string) {
	s.mu.Lock()
	s.state = SyncError
	s.message = msg
	s.lastError = msg
	s.mu.Unlock()
	s.notifySubscribers()
}

// setOffline transitions to offline state with backoff tracking.
func (s *SyncManager) setOffline() {
	s.mu.Lock()
	s.consecutiveFailures++
	s.state = SyncOffline
	s.message = "Offline"
	s.lastError = "Remote unreachable"
	s.mu.Unlock()
	s.notifySubscribers()
}

// Subscribe registers an SSE subscriber and returns a channel + unsubscribe ID.
func (s *SyncManager) Subscribe() (_ int, _ <-chan SyncStatus) {
	ch := make(chan SyncStatus, 8)
	s.subMu.Lock()
	s.subID++
	id := s.subID
	s.subs[id] = ch
	s.subMu.Unlock()
	return id, ch
}

// Unsubscribe removes a subscriber by ID and closes the channel.
func (s *SyncManager) Unsubscribe(id int) {
	s.subMu.Lock()
	if ch, ok := s.subs[id]; ok {
		delete(s.subs, id)
		close(ch)
	}
	s.subMu.Unlock()
}

// notifySubscribers sends the current status to all SSE subscribers (non-blocking).
func (s *SyncManager) notifySubscribers() {
	status := s.Status()
	s.subMu.Lock()
	for _, ch := range s.subs {
		select {
		case ch <- status:
		default:
			// Drop if subscriber is slow
		}
	}
	s.subMu.Unlock()
}

// setConflict transitions to conflict state and builds a ConflictSet from git.
func (s *SyncManager) setConflict() {
	branch := s.readBranch()

	// Try to build a structured conflict set from git
	cs, err := BuildConflictSetFromGit(s.backend, branch)
	if err != nil {
		log.Printf("Sync: failed to build conflict set: %v", err)
	}

	s.mu.Lock()
	s.state = SyncConflict
	s.message = "Sync conflict"
	s.lastError = "Rebase conflict — resolve conflicts to continue"
	if cs != nil && len(cs.Files) > 0 {
		s.conflicts = cs
		s.message = fmt.Sprintf("Sync conflict (%d files)", len(cs.Files))
	}
	s.mu.Unlock()
	s.notifySubscribers()
}

// Conflicts returns the current conflict set (thread-safe).
func (s *SyncManager) Conflicts() *ConflictSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conflicts
}

// ClearConflicts clears the conflict set and resets state.
func (s *SyncManager) ClearConflicts() {
	s.mu.Lock()
	s.conflicts = nil
	s.state = SyncAhead
	s.message = "Conflicts resolved"
	s.lastError = ""
	s.mu.Unlock()
}

// CompleteMerge creates a merge commit with the resolved files, then pushes.
// This is called after all conflicts have been individually resolved and their
// files written to disk + staged with git add.
func (s *SyncManager) CompleteMerge() error {
	if !s.enabled {
		return fmt.Errorf("git not enabled")
	}

	branch := s.readBranch()
	upstream := "origin/" + branch

	// Stage any remaining changes
	if err := s.backend.StageAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Create the merge commit
	if err := s.backend.Commit("rela: resolve merge conflicts"); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Clear conflict state
	s.ClearConflicts()

	// Now try to rebase on top of upstream and push
	// First check if we're still behind
	behind := s.countRevs("HEAD.." + upstream)
	if behind > 0 {
		// Rebase our resolution commit on top of upstream
		if err := s.backend.Rebase(upstream); err != nil {
			_ = s.backend.AbortRebase()
			s.setConflict()
			return fmt.Errorf("rebase after merge: %w", err)
		}
	}

	// Push
	if err := s.pushNow(); err != nil {
		return err
	}

	s.mu.Lock()
	s.lastSync = time.Now()
	s.lastError = ""
	s.mu.Unlock()

	s.updateState()
	return nil
}

// readBranch reads the current branch name from git.
func (s *SyncManager) readBranch() string {
	branch, err := s.backend.CurrentBranch()
	if err != nil {
		return "unknown"
	}
	return branch
}

// countRevs counts revisions in a rev-list range (e.g. "A..B").
func (s *SyncManager) countRevs(revRange string) int {
	n, err := s.backend.RevCount(revRange)
	if err != nil {
		return 0
	}
	return n
}

// Backend returns the underlying GitBackend (for conflict resolution and other
// code that needs direct git access).
func (s *SyncManager) Backend() GitBackend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backend
}

// CommitMessage builds a structured commit message for an entity operation.
func CommitMessage(op, entityID, detail string) string {
	if detail != "" {
		return fmt.Sprintf("rela: %s %s %s", op, entityID, detail)
	}
	return fmt.Sprintf("rela: %s %s", op, entityID)
}

// CommitMessageCreate builds a commit message for entity creation.
func CommitMessageCreate(entityID, title string) string {
	if title != "" {
		return CommitMessage("create", entityID, fmt.Sprintf("%q", title))
	}
	return CommitMessage("create", entityID, "")
}

// CommitMessageUpdate builds a commit message for entity update.
func CommitMessageUpdate(entityID string, changedFields []string) string {
	if len(changedFields) > 0 {
		return CommitMessage("update", entityID, "("+strings.Join(changedFields, ", ")+")")
	}
	return CommitMessage("update", entityID, "")
}

// CommitMessageDelete builds a commit message for entity deletion.
func CommitMessageDelete(entityID string) string {
	return CommitMessage("delete", entityID, "")
}

// SetOnPull sets the callback invoked after a fast-forward pull.
// This allows setting the callback after construction (e.g. when the App
// instance isn't available yet at SyncManager creation time).
func (s *SyncManager) SetOnPull(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onPull = fn
}

// AbsPath returns the absolute path for a file relative to the repo root.
func (s *SyncManager) AbsPath(relPath string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.backend == nil {
		return relPath
	}
	return filepath.Join(s.backend.RepoRoot(), relPath)
}
