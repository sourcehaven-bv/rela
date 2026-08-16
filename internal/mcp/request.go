package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Argument-access shim for the go-sdk migration (TKT-UIR41P).
//
// The 34 handlers pluck arguments through RequireString / GetString / GetInt /
// GetBool / GetStringSlice, which mark3labs hung off its CallToolRequest. The
// go-sdk instead unmarshals into a typed In struct via the generic AddTool.
//
// Porting all 26 tools to typed structs in the same change as the transport
// swap would mean rewriting every tool's schema AND its decode path at once —
// two independent sources of behavioral drift, landing together, in the one PR
// whose entire purpose is "no behavior change". So this shim reproduces the old
// accessors over the go-sdk's raw json.RawMessage arguments, byte-for-byte
// including the error strings (which the committed goldens pin).
//
// Moving to typed In structs is a worthwhile follow-up — it buys real schema
// validation — but it is a separate, independently reviewable step.

// toolRequest wraps a go-sdk CallToolRequest and lazily decodes its raw
// arguments into the untyped map the handlers expect.
type toolRequest struct {
	req  *mcpgo.CallToolRequest
	args map[string]any
}

// newToolRequest decodes the raw arguments once per call. A malformed or
// absent payload yields a nil map, which every accessor below treats as
// "argument missing" — matching mark3labs' GetArguments, which returned nil
// when Arguments was not a map.
func newToolRequest(req *mcpgo.CallToolRequest) toolRequest {
	tr := toolRequest{req: req}
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return tr
	}
	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return tr
	}
	tr.args = args
	return tr
}

// GetArguments returns the decoded argument map, or nil.
func (r toolRequest) GetArguments() map[string]any { return r.args }

// RequireString returns a string argument, erroring when absent or mistyped.
// The two error strings are reproduced verbatim from mark3labs because they
// reach the model in tool-error results and are pinned by the goldens.
func (r toolRequest) RequireString(key string) (string, error) {
	if val, ok := r.args[key]; ok {
		if str, ok := val.(string); ok {
			return str, nil
		}
		return "", fmt.Errorf("argument %q is not a string", key)
	}
	return "", fmt.Errorf("required argument %q not found", key)
}

// GetString returns a string argument or defaultValue. A present-but-mistyped
// value falls back to the default rather than erroring (mark3labs semantics).
func (r toolRequest) GetString(key, defaultValue string) string {
	if val, ok := r.args[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetInt returns an int argument or defaultValue, accepting int, float64 (the
// JSON number case) and a numeric string.
func (r toolRequest) GetInt(key string, defaultValue int) int {
	if val, ok := r.args[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// GetBool returns a bool argument or defaultValue, accepting bool, a parseable
// string, and int/float64 (non-zero is true).
func (r toolRequest) GetBool(key string, defaultValue bool) bool {
	if val, ok := r.args[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		case int:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return defaultValue
}

// GetStringSlice returns a string-slice argument or defaultValue. Non-string
// members of a []any are skipped, not an error (mark3labs semantics).
func (r toolRequest) GetStringSlice(key string, defaultValue []string) []string {
	if val, ok := r.args[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []any:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return defaultValue
}
