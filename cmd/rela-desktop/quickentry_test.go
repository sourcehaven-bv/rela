package main

import (
	"errors"
	"sync"
	"testing"
)

type fakeRegistrar struct {
	mu   sync.Mutex
	regs map[string]func()
	fail bool
}

func (f *fakeRegistrar) Register(a string, cb func()) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("taken")
	}
	if f.regs == nil {
		f.regs = map[string]func(){}
	}
	f.regs[a] = cb
	return nil
}

func (f *fakeRegistrar) Unregister(a string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.regs, a)
	return nil
}

func (f *fakeRegistrar) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k := range f.regs {
		out = append(out, k)
	}
	return out
}

type fakePopup struct {
	focused, closed int
	onClose         func()
}

func (p *fakePopup) Focus() { p.focused++ }
func (p *fakePopup) Close() {
	p.closed++
	p.onClose() // a real window runs its close handler while closing
}

func newTestHotkey() (*quickEntryHotkey, *fakeRegistrar, *[]*fakePopup) {
	reg := &fakeRegistrar{}
	var opened []*fakePopup
	q := &quickEntryHotkey{reg: reg}
	q.open = func(_ string, onClose func()) popupWindow {
		p := &fakePopup{onClose: onClose}
		opened = append(opened, p)
		return p
	}
	return q, reg, &opened
}

func TestQuickEntryHotkey(t *testing.T) {
	t.Run("registers and replaces the shortcut", func(t *testing.T) {
		q, reg, _ := newTestHotkey()
		q.sync(fixed("CmdOrCtrl+Shift+Space"))
		q.sync(fixed("Ctrl+Alt+N"))
		if got := reg.keys(); len(got) != 1 || got[0] != "Ctrl+Alt+N" {
			t.Fatalf("registered %v", got)
		}
		q.sync(fixed(""))
		if got := reg.keys(); len(got) != 0 {
			t.Fatalf("still registered %v", got)
		}
	})

	t.Run("second press focuses the open popup", func(t *testing.T) {
		q, _, opened := newTestHotkey()
		q.sync(fixed("Ctrl+Q"))
		q.trigger()
		q.trigger()
		if len(*opened) != 1 || (*opened)[0].focused != 1 {
			t.Fatalf("opened %d popups", len(*opened))
		}
	})

	t.Run("a closed popup opens again", func(t *testing.T) {
		q, _, opened := newTestHotkey()
		q.trigger()
		(*opened)[0].Close()
		q.trigger()
		if len(*opened) != 2 {
			t.Fatalf("opened %d popups", len(*opened))
		}
	})

	t.Run("a project switch closes the popup", func(t *testing.T) {
		q, _, opened := newTestHotkey()
		q.sync(fixed("Ctrl+Q"))
		q.trigger()
		q.sync(fixed("Ctrl+Q"))
		if (*opened)[0].closed != 1 {
			t.Fatal("popup of the previous project stayed open")
		}
		q.trigger()
		if len(*opened) != 2 {
			t.Fatalf("opened %d popups", len(*opened))
		}
	})

	t.Run("a popup opened during a switch is closed", func(t *testing.T) {
		q, _, _ := newTestHotkey()
		var late *fakePopup
		q.open = func(_ string, onClose func()) popupWindow {
			q.sync(fixed("")) // the project changes while the window is being made
			late = &fakePopup{onClose: onClose}
			return late
		}
		q.trigger()
		if late.closed != 1 {
			t.Fatal("stale popup stayed open")
		}
	})

	t.Run("a taken shortcut is not recorded", func(t *testing.T) {
		q, reg, _ := newTestHotkey()
		reg.fail = true
		q.sync(fixed("Ctrl+Q"))
		if q.current != "" {
			t.Fatalf("current = %q", q.current)
		}
	})
}

func TestQuickEntryHotkey_ConcurrentTriggers(t *testing.T) {
	q, _, _ := newTestHotkey()
	var mu sync.Mutex
	n := 0
	q.open = func(string, func()) popupWindow {
		mu.Lock()
		n++
		mu.Unlock()
		return quietPopup{}
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(q.trigger)
	}
	wg.Wait()
	if n != 1 {
		t.Fatalf("opened %d popups", n)
	}
}

type quietPopup struct{}

func (quietPopup) Focus() {}
func (quietPopup) Close() {}

func fixed(accel string) func() (string, string) {
	return func() (string, string) { return accel, "/p/x/quick?popup=1" }
}
