package domain

import "testing"

func TestCoverageShowsPassage(t *testing.T) {
	const defaultCap CoverageCap = 10
	q := Quote{PassageWords: 4}
	tests := []struct {
		name     string
		coverage Coverage
		want     bool
	}{
		{"nothing public yet", Coverage{}, true},
		{"fits under the cap", Coverage{Words: 5}, true},
		{"reaches the cap exactly", Coverage{Words: 6}, true},
		{"one word past the cap", Coverage{Words: 7}, false},
		{"already past the cap", Coverage{Words: 40}, false},
		{"an identical passage is already shown", Coverage{Words: 40, IdenticalShown: true}, true},
		{"the work's own cap is higher", Coverage{Words: 7, CapOverride: ptr(11)}, true},
		{"the work's own cap is lower", Coverage{Words: 0, CapOverride: ptr(3)}, false},
		{"the work's own cap is reached exactly", Coverage{Words: 1, CapOverride: ptr(5)}, true},
		{"the work's own cap of zero hides every new passage", Coverage{CapOverride: ptr(0)}, false},
		{"zero hides nothing that is already shown", Coverage{CapOverride: ptr(0), IdenticalShown: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.coverage.ShowsPassage(q, defaultCap); got != tt.want {
				t.Errorf("ShowsPassage() = %v, want %v", got, tt.want)
			}
		})
	}
}
