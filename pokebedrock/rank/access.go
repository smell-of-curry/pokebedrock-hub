package rank

// CanAccessBeta reports whether the given ranks may use beta/dev servers.
// Matches the BetaLock join gate: Supporter (any) or Moderator+.
func CanAccessBeta(ranks []Rank) bool {
	var highest Rank
	hasSupporter := false
	for _, r := range ranks {
		if r == Supporter {
			hasSupporter = true
		}
		if r > highest {
			highest = r
		}
	}
	return hasSupporter || highest >= Moderator
}
