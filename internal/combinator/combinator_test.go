package combinator_test

import (
	"testing"

	"github.com/otterlab-bio/matsrun/internal/combinator"
	"github.com/otterlab-bio/matsrun/internal/types"
)

func TestPairwise_ThreeGroups(t *testing.T) {
	combos := combinator.Pairwise([]string{"A", "B", "C"})
	want := []types.GroupCombination{
		{Group1: "A", Group2: "B"},
		{Group1: "A", Group2: "C"},
		{Group1: "B", Group2: "C"},
	}
	if len(combos) != len(want) {
		t.Fatalf("got %d combinations, want %d", len(combos), len(want))
	}
	for i, got := range combos {
		if got != want[i] {
			t.Errorf("combo[%d] = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestPairwise_TwoGroups(t *testing.T) {
	combos := combinator.Pairwise([]string{"Ctrl", "Treat"})
	if len(combos) != 1 {
		t.Fatalf("got %d combinations, want 1", len(combos))
	}
	if combos[0].Group1 != "Ctrl" || combos[0].Group2 != "Treat" {
		t.Errorf("unexpected combination: %+v", combos[0])
	}
}

func TestPairwise_Deduplication(t *testing.T) {
	// Duplicate entries should produce the same result as unique entries.
	combos := combinator.Pairwise([]string{"A", "B", "A", "B"})
	if len(combos) != 1 {
		t.Fatalf("got %d combinations, want 1 (after dedup)", len(combos))
	}
}

func TestPairwise_Deterministic(t *testing.T) {
	// Regardless of input order, output should be sorted.
	c1 := combinator.Pairwise([]string{"Z", "A", "M"})
	c2 := combinator.Pairwise([]string{"M", "Z", "A"})
	if len(c1) != len(c2) {
		t.Fatal("length mismatch")
	}
	for i := range c1 {
		if c1[i] != c2[i] {
			t.Errorf("index %d: %+v != %+v", i, c1[i], c2[i])
		}
	}
}

func TestPairwise_SingleGroup(t *testing.T) {
	combos := combinator.Pairwise([]string{"OnlyGroup"})
	if len(combos) != 0 {
		t.Errorf("expected 0 combinations for single group, got %d", len(combos))
	}
}
