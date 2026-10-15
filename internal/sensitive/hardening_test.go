package sensitive

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnknownContainersMaskEveryPriorLeaf(t *testing.T) {
	const canary = "synthetic-mask-canary"
	for _, tc := range []struct{ before, mask any }{
		{canary, true},
		{map[string]any{"child": canary}, map[string]any{"child": true}},
		{[]any{canary}, []any{true}},
		{map[string]any{"api_token": canary}, nil},
	} {
		for _, afterMask := range []bool{false, true} {
			bm, am := tc.mask, any(nil)
			if afterMask {
				bm, am = nil, tc.mask
			}
			got := Diff(tc.before, nil, bm, am, true)
			b, _ := json.Marshal(got)
			if strings.Contains(string(b), canary) {
				t.Fatal("unknown container disclosed prior sensitive leaf")
			}
		}
	}
	if !IsSensitive(true, []any{"parent", 0, "leaf"}) || !IsSensitive(map[string]any{"parent": true}, []any{"parent", 0, "leaf"}) {
		t.Fatal("parent mask must cover all descendants")
	}
}

func FuzzUnknownSensitiveValues(f *testing.F) {
	f.Add("synthetic-fuzz-canary")
	f.Fuzz(func(t *testing.T, value string) {
		got := Diff(map[string]any{"payload": value}, map[string]any{}, map[string]any{"payload": true}, nil, map[string]any{"payload": true})
		if len(got) != 1 || got[0].Before != Redacted || got[0].After != Unknown || !got[0].Sensitive {
			t.Fatal("sensitive/unknown invariant violated")
		}
	})
}
