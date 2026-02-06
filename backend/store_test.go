package main

import (
	"testing"
	"time"
)

func TestAdvanceRecurringDateMonthlyKeepsAnchorDay(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.January, 31, 12, 0, 0, 0, time.UTC)
	anchorDay := start.Day()

	feb := advanceRecurringDate(start, "monthly", anchorDay)
	if feb.Year() != 2026 || feb.Month() != time.February || feb.Day() != 28 {
		t.Fatalf("expected Feb 28, 2026; got %s", feb.Format("2006-01-02"))
	}

	mar := advanceRecurringDate(feb, "monthly", anchorDay)
	if mar.Year() != 2026 || mar.Month() != time.March || mar.Day() != 31 {
		t.Fatalf("expected Mar 31, 2026; got %s", mar.Format("2006-01-02"))
	}
}
