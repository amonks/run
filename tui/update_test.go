package tui

import "testing"

func TestScrollMenu_FirstScrollDownSkipsIndicatorOnlyStep(t *testing.T) {
	// From offset 0, incrementing offset by 1 would just replace the top
	// task with the ▲ indicator without exposing a new task at the bottom
	// (visible shrinks from [0,9) to [1,9)). Skip straight to offset 2 so
	// the wheel tick actually reveals index 9.
	offset, selected := scrollMenu(0, 0, 20, 10, 1)
	if offset != 2 {
		t.Errorf("offset: got %d, want 2", offset)
	}
	// selected=0 is now above the viewport [2,10); nudge to 2.
	if selected != 2 {
		t.Errorf("selected: got %d, want 2", selected)
	}
}

func TestScrollMenu_PreservesSelectionWhenStillInViewport(t *testing.T) {
	// After skipping the indicator-only step, offset=2 and the viewport is
	// [2, 10); selected=5 stays put.
	offset, selected := scrollMenu(0, 5, 20, 10, 1)
	if offset != 2 {
		t.Errorf("offset: got %d, want 2", offset)
	}
	if selected != 5 {
		t.Errorf("selected: got %d, want 5 (unchanged)", selected)
	}
}

func TestScrollMenu_NormalScrollDownAdvancesByOne(t *testing.T) {
	// Once the ▲ indicator is already visible, each subsequent wheel tick
	// advances by exactly one row.
	offset, _ := scrollMenu(2, 5, 20, 10, 1)
	if offset != 3 {
		t.Errorf("offset: got %d, want 3", offset)
	}
}

func TestScrollMenu_ClampsAtMaxOffset(t *testing.T) {
	// maxOffset for total=20, height=10 is 20-10+1 = 11.
	offset, _ := scrollMenu(11, 15, 20, 10, 1)
	if offset != 11 {
		t.Errorf("offset: got %d, want 11 (clamped)", offset)
	}
}

func TestScrollMenu_ClampsAtZero(t *testing.T) {
	offset, selected := scrollMenu(0, 5, 20, 10, -1)
	if offset != 0 {
		t.Errorf("offset: got %d, want 0 (clamped)", offset)
	}
	if selected != 5 {
		t.Errorf("selected: got %d, want 5 (unchanged)", selected)
	}
}

func TestScrollMenu_ScrollUpSkipsOffset1(t *testing.T) {
	// Scrolling up from offset=2 would naively land at offset=1 — a state
	// strictly worse than offset=0 (same bottom, just hides task 0 behind
	// the ▲ indicator). Snap through to offset=0.
	offset, _ := scrollMenu(2, 5, 20, 10, -1)
	if offset != 0 {
		t.Errorf("offset: got %d, want 0", offset)
	}
}

func TestScrollMenu_ScrollUpNudgesSelectionIntoView(t *testing.T) {
	// offset 5, selected 12 (bottom of visible [5, 13)); scroll up by 1.
	// New offset 4, visible [4, 12); selected 12 falls off bottom, nudged to 11.
	offset, selected := scrollMenu(5, 12, 20, 10, -1)
	if offset != 4 {
		t.Errorf("offset: got %d, want 4", offset)
	}
	if selected != 11 {
		t.Errorf("selected: got %d, want 11 (nudged to stay in viewport)", selected)
	}
}

func TestScrollMenu_AllTasksFit_IsNoOp(t *testing.T) {
	// When the whole list fits, there's nothing to scroll and the wheel
	// must not disturb the selection.
	offset, selected := scrollMenu(0, 5, 8, 10, 1)
	if offset != 0 || selected != 5 {
		t.Errorf("got offset=%d selected=%d, want 0/5 (unchanged)", offset, selected)
	}
	offset, selected = scrollMenu(0, 5, 8, 10, -1)
	if offset != 0 || selected != 5 {
		t.Errorf("got offset=%d selected=%d, want 0/5 (unchanged)", offset, selected)
	}
}
