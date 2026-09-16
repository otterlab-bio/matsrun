// Package combinator generates pairwise group combinations.
package combinator

import (
	"sort"

	"github.com/otterlab-bio/matsrun/internal/types"
)

// Pairwise returns all C(n,2) ordered pairs from the unique values in groups.
// The slice is sorted for deterministic output.
// This fixes the R bug: `for (a in length(combinations))` which only visited
// the last element — here we iterate over all combinations.
func Pairwise(groups []string) []types.GroupCombination {
	// Deduplicate and sort for determinism.
	seen := make(map[string]struct{}, len(groups))
	unique := make([]string, 0, len(groups))
	for _, g := range groups {
		if _, ok := seen[g]; !ok {
			seen[g] = struct{}{}
			unique = append(unique, g)
		}
	}
	sort.Strings(unique)

	var result []types.GroupCombination
	for i := 0; i < len(unique); i++ {
		for j := i + 1; j < len(unique); j++ {
			result = append(result, types.GroupCombination{
				Group1: unique[i],
				Group2: unique[j],
			})
		}
	}
	return result
}
