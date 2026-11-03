package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// validProps is a message's properties after validation: every value in
// its stored form, every name declared by the message type.
type validProps struct {
	props map[string]any
	// size is the length of the JSON encoding of props.
	size int
	// refs lists the entity ids the ref properties name, sorted.
	refs []string
}

// validateProps checks in against mt with
// [realtimecfg.MessageType.NormalizeProps] and returns the stored form. It
// maps the refusals to denials: more than [channels.MaxProps] properties is
// 400 too_many_props, an encoding over maxBytes is 413, and every other
// refusal is 400 invalid.
func validateProps(mt *realtimecfg.MessageType, in map[string]any, maxBytes int) (validProps, error) {
	np, err := mt.NormalizeProps(in, maxBytes)
	switch {
	case errors.Is(err, realtimecfg.ErrTooManyProps):
		return validProps{}, deny(http.StatusBadRequest, TypeTooManyProps,
			fmt.Sprintf("a message has at most %d properties", channels.MaxProps))
	case errors.Is(err, realtimecfg.ErrTooLarge):
		return validProps{}, deny(http.StatusRequestEntityTooLarge, TypeTooLarge,
			fmt.Sprintf("the message properties are over %d bytes", maxBytes))
	case err != nil:
		return validProps{}, invalid("%s", err.Error())
	}
	return validProps{props: np.Props, size: np.Size, refs: np.Refs}, nil
}

// whenHolds evaluates mt's `when:` over props in their stored form. A
// program that fails to evaluate does not hold.
func whenHolds(ctx context.Context, mt *realtimecfg.MessageType, props map[string]any, now time.Time) bool {
	if mt.When == nil {
		return true
	}
	return mt.Holds(ctx, mt.PredicateFields(props), now)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
