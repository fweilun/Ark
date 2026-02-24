// README: Unit tests for matching service helpers.
package matching

import (
	"testing"

	"ark/internal/types"
)

func TestSelectRandom_LessOrEqual(t *testing.T) {
	ids := []types.ID{"a", "b", "c"}
	result := selectRandom(ids, 5)
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
	// original should be unchanged
	if ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Errorf("selectRandom must not mutate input slice: %v", ids)
	}
}

func TestSelectRandom_Truncates(t *testing.T) {
	ids := make([]types.ID, 20)
	for i := range ids {
		ids[i] = types.ID(string(rune('a' + i)))
	}
	result := selectRandom(ids, 10)
	if len(result) != 10 {
		t.Errorf("expected 10 items, got %d", len(result))
	}
}

func TestSelectRandom_Empty(t *testing.T) {
	result := selectRandom(nil, 10)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

func TestSelectRandom_ContainsOriginals(t *testing.T) {
	ids := []types.ID{"x", "y", "z", "w", "v"}
	result := selectRandom(ids, 3)
	// All selected IDs must be from the original set.
	origSet := map[types.ID]struct{}{"x": {}, "y": {}, "z": {}, "w": {}, "v": {}}
	for _, id := range result {
		if _, ok := origSet[id]; !ok {
			t.Errorf("unexpected id %q in result", id)
		}
	}
}

func TestSelectRandom_NoDuplicates(t *testing.T) {
	ids := []types.ID{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	result := selectRandom(ids, 10)
	seen := map[types.ID]struct{}{}
	for _, id := range result {
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate id %q in result", id)
		}
		seen[id] = struct{}{}
	}
}
