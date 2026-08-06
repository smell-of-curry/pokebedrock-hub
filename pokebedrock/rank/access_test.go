package rank

import "testing"

func TestCanAccessBeta(t *testing.T) {
	tests := []struct {
		name  string
		ranks []Rank
		want  bool
	}{
		{"empty", nil, false},
		{"unlinked", []Rank{UnLinked}, false},
		{"trainer", []Rank{Trainer}, false},
		{"booster", []Rank{ServerBooster}, false},
		{"supporter", []Rank{Supporter}, true},
		{"premium alone", []Rank{Premium}, false},
		{"helper", []Rank{Helper}, false},
		{"moderator", []Rank{Moderator}, true},
		{"admin", []Rank{Admin}, true},
		{"owner", []Rank{Owner}, true},
		{"trainer+supporter", []Rank{Trainer, Supporter}, true},
		{"premium+supporter", []Rank{Premium, Supporter}, true},
		{"helper+moderator", []Rank{Helper, Moderator}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanAccessBeta(tt.ranks); got != tt.want {
				t.Fatalf("CanAccessBeta(%v) = %v, want %v", tt.ranks, got, tt.want)
			}
		})
	}
}
