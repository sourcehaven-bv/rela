package docscapture

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

//go:embed focus.js
var focusJS string

// pageSize returns the full scrollable page dimensions in CSS px.
func pageSize(ctx context.Context) (rect, error) {
	var d struct{ W, H float64 }
	err := chromedp.Evaluate(
		`({W: Math.ceil(document.documentElement.scrollWidth),
		   H: Math.ceil(Math.max(document.body.scrollHeight, document.documentElement.scrollHeight))})`,
		&d,
	).Do(ctx)
	return rect{W: d.W, H: d.H}, err
}

// resolveClip turns a clip spec into a page-coordinate rect. The spec is either
// a predefined keyword ("focus" — the bounding box of the annotated targets) or
// a CSS selector (that element's box). Fails loud if nothing matches.
func resolveClip(ctx context.Context, clip string, annotations []docs.Annotation) (rect, error) {
	if clip == clipFocus {
		return focusRect(ctx, annotations)
	}
	return selectorRect(ctx, clip)
}

const clipFocus = "focus"

// selectorRect returns the page-coordinate box of the first element matching a
// CSS selector.
func selectorRect(ctx context.Context, selector string) (rect, error) {
	sel, err := json.Marshal(selector)
	if err != nil {
		return rect{}, err
	}
	var r *rect
	expr := fmt.Sprintf(`(function(){var e=document.querySelector(%s);if(!e)return null;`+
		`var b=e.getBoundingClientRect();`+
		`return {X:b.left+window.scrollX,Y:b.top+window.scrollY,W:b.width,H:b.height};})()`, sel)
	if err := chromedp.Evaluate(expr, &r).Do(ctx); err != nil {
		return rect{}, err
	}
	if r == nil || r.W == 0 || r.H == 0 {
		return rect{}, fmt.Errorf("clip selector %q matched no visible element", selector)
	}
	return *r, nil
}

// focusRect returns the union bounding box of every annotation's target element,
// so clip="focus" crops to what the arrows point at. Requires annotations.
func focusRect(ctx context.Context, annotations []docs.Annotation) (rect, error) {
	if len(annotations) == 0 {
		return rect{}, errors.New(`clip="focus" needs at least one arrow/box to focus on`)
	}
	selectors := make([]string, 0, len(annotations))
	for _, a := range annotations {
		sel, err := anchorSelector(a.At)
		if err != nil {
			return rect{}, err
		}
		selectors = append(selectors, sel)
	}
	data, err := json.Marshal(selectors)
	if err != nil {
		return rect{}, err
	}
	var r *rect
	expr := strings.ReplaceAll(focusJS, "__SELS__", string(data))
	if err := chromedp.Evaluate(expr, &r).Do(ctx); err != nil {
		return rect{}, err
	}
	if r == nil {
		return rect{}, fmt.Errorf(`clip="focus": none of the annotation targets (%s) matched a visible element`, strings.Join(selectors, ", "))
	}
	return *r, nil
}

// padAndClamp expands a region by pad on all sides and clamps it to the page so
// the crop never extends past the content (no white margin beyond the page).
func padAndClamp(r rect, pad float64, page rect) rect {
	x := r.X - pad
	y := r.Y - pad
	right := r.X + r.W + pad
	bottom := r.Y + r.H + pad
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if right > page.W {
		right = page.W
	}
	if bottom > page.H {
		bottom = page.H
	}
	return rect{X: x, Y: y, W: right - x, H: bottom - y}
}
