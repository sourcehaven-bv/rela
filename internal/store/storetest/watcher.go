package storetest

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const eventTimeout = 50 * time.Millisecond

// RunWatcherTests runs event subscription conformance tests.
//
//nolint:funlen // subtests-as-table make this naturally long but readable

func RunWatcherTests(t *testing.T, f Factory) {
	t.Run("ReceivesEntityCreated", func(t *testing.T) {
		s := f(t)
		events, cancel := s.Subscribe(10)
		defer cancel()

		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		select {
		case ev := <-events:
			assert.Equal(t, store.EventEntityCreated, ev.Op)
			assert.Equal(t, "T-1", ev.EntityID)
			assert.Equal(t, "ticket", ev.EntityType)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("ReceivesEntityUpdated", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		require.NoError(t, s.CreateEntity(ctx(), e))

		events, cancel := s.Subscribe(10)
		defer cancel()

		e.SetString("title", "Updated")
		require.NoError(t, s.UpdateEntity(ctx(), e))

		select {
		case ev := <-events:
			assert.Equal(t, store.EventEntityUpdated, ev.Op)
			assert.Equal(t, "T-1", ev.EntityID)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("ReceivesEntityDeleted", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		events, cancel := s.Subscribe(10)
		defer cancel()

		_, err := s.DeleteEntity(ctx(), "T-1", false)
		require.NoError(t, err)

		select {
		case ev := <-events:
			assert.Equal(t, store.EventEntityDeleted, ev.Op)
			assert.Equal(t, "T-1", ev.EntityID)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("ReceivesRelationEvents", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))

		events, cancel := s.Subscribe(10)
		defer cancel()

		_, err := s.CreateRelation(ctx(), "A", "requires", "B", nil)
		require.NoError(t, err)

		select {
		case ev := <-events:
			assert.Equal(t, store.EventRelationCreated, ev.Op)
			assert.Equal(t, "A", ev.From)
			assert.Equal(t, "requires", ev.RelationType)
			assert.Equal(t, "B", ev.To)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("DropsWhenFull", func(t *testing.T) {
		s := f(t)

		events, cancel := s.Subscribe(1)
		defer cancel()

		for i := 0; i < 5; i++ {
			e := entity.New("T-"+string(rune('A'+i)), "ticket")
			require.NoError(t, s.CreateEntity(ctx(), e))
		}

		var received int
		for {
			select {
			case <-events:
				received++
			case <-time.After(eventTimeout):
				goto done
			}
		}
	done:
		assert.Equal(t, 1, received)
	})

	t.Run("CancelStopsEvents", func(t *testing.T) {
		s := f(t)
		events, cancel := s.Subscribe(10)
		cancel()

		_, ok := <-events
		assert.False(t, ok)
	})

	t.Run("MultipleSubscribers", func(t *testing.T) {
		s := f(t)
		events1, cancel1 := s.Subscribe(10)
		defer cancel1()
		events2, cancel2 := s.Subscribe(10)
		defer cancel2()

		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		for _, events := range []<-chan store.Event{events1, events2} {
			select {
			case ev := <-events:
				assert.Equal(t, store.EventEntityCreated, ev.Op)
			case <-time.After(100 * time.Millisecond):
				t.Fatal("timed out waiting for event")
			}
		}
	})

	t.Run("DoubleCancelSafe", func(t *testing.T) {
		s := f(t)
		_, cancel := s.Subscribe(10)
		cancel()
		cancel() // should not panic
	})

	t.Run("CancelOneKeepsOther", func(t *testing.T) {
		s := f(t)
		events1, cancel1 := s.Subscribe(10)
		events2, cancel2 := s.Subscribe(10)
		defer cancel2()

		cancel1()

		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		select {
		case ev := <-events2:
			assert.Equal(t, store.EventEntityCreated, ev.Op)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("second subscriber should still receive events")
		}

		_, ok := <-events1
		assert.False(t, ok)
	})

	t.Run("CloseClosesSubscriberChannels", func(t *testing.T) {
		s := f(t)
		events, _ := s.Subscribe(10)

		require.NoError(t, s.Close())

		_, ok := <-events
		assert.False(t, ok)
	})

	t.Run("CascadeDeleteEmitsRelationEvents", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, _ = s.CreateRelation(ctx(), "A", "requires", "B", nil)

		events, cancel := s.Subscribe(10)
		defer cancel()

		_, err := s.DeleteEntity(ctx(), "A", true)
		require.NoError(t, err)

		var ops []store.EventOp
		for {
			select {
			case ev := <-events:
				ops = append(ops, ev.Op)
			case <-time.After(eventTimeout):
				goto done
			}
		}
	done:
		assert.Contains(t, ops, store.EventEntityDeleted)
		assert.Contains(t, ops, store.EventRelationDeleted)
	})
}
