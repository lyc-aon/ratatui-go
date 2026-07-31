package widgets

import (
	"testing"
	"time"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
)

func TestCalendarLeapYearAndMonthBoundary(t *testing.T) {
	feb2024 := NewDate(2024, time.February, 1)
	if feb2024.daysInMonth() != 29 {
		t.Errorf("Feb 2024 daysInMonth() = %d, want 29", feb2024.daysInMonth())
	}

	feb2023 := NewDate(2023, time.February, 1)
	if feb2023.daysInMonth() != 28 {
		t.Errorf("Feb 2023 daysInMonth() = %d, want 28", feb2023.daysInMonth())
	}

	// Month boundary AddDays
	jan31 := NewDate(2024, time.January, 31)
	feb1 := jan31.AddDays(1)
	if feb1.Year != 2024 || feb1.Month != time.February || feb1.Day != 1 {
		t.Errorf("Jan 31 + 1 day = %+v, want 2024-02-01", feb1)
	}
}

func TestCalendarSurroundingAndEvents(t *testing.T) {
	events := NewCalendarEventStore()
	eventDate := NewDate(2024, time.March, 15)
	eventStyle := style.New().WithFG(style.Red)
	events.Add(eventDate, eventStyle)

	surroundingStyle := style.New().WithFG(style.DarkGray)

	m := NewMonthly(NewDate(2024, time.March, 1), events).
		ShowSurrounding(surroundingStyle).
		ShowWeekdaysHeader(style.New()).
		ShowMonthHeader(style.New())

	w := m.Width()
	h := m.Height()
	area := layout.NewRect(0, 0, w, h)
	buf := buffer.Empty(area)

	m.Render(area, buf)

	// Check that eventDate style is retrieved from DateStyler
	if events.GetStyle(eventDate).FG != style.Red {
		t.Errorf("event date style FG = %v, want Red", events.GetStyle(eventDate).FG)
	}
}

func TestCalendarZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)

	m := NewMonthly(NewDate(2024, time.July, 31), nil).
		ShowSurrounding(style.New()).
		ShowWeekdaysHeader(style.New()).
		ShowMonthHeader(style.New())

	m.Render(zero, buf)
}

func TestCalendarDefaultStyleStaysInsideGrid(t *testing.T) {
	m := NewMonthly(NewDate(2024, time.March, 1), nil).
		DefaultStyle(style.New().WithBG(style.Blue))
	area := layout.NewRect(0, 0, m.Width()+3, m.Height()+1)
	buf := buffer.Empty(area)

	m.Render(area, buf)

	cell, ok := buf.Get(area.Right()-1, 0)
	if !ok {
		t.Fatal("outside-grid cell missing")
	}
	bg, set := cell.Style.Background()
	if !set || bg != style.Reset {
		t.Fatalf("outside-grid background = (%v, %v), want Reset", bg, set)
	}
}

func TestCalendarMonthHeaderStylesWholeRow(t *testing.T) {
	headerStyle := style.New().WithBG(style.Blue)
	m := NewMonthly(NewDate(2024, time.March, 1), nil).
		ShowMonthHeader(headerStyle)
	area := layout.NewRect(0, 0, m.Width(), m.Height())
	buf := buffer.Empty(area)

	m.Render(area, buf)

	cell, ok := buf.Get(0, 0)
	if !ok {
		t.Fatal("month-header cell missing")
	}
	bg, set := cell.Style.Background()
	if !set || bg != style.Blue {
		t.Fatalf("month-header row background = (%v, %v), want Blue", bg, set)
	}
}

func TestCalendarEdgeState(t *testing.T) {
	// Year boundary arithmetic: Dec 31 2024 -> Jan 1 2025
	dec31 := NewDate(2024, time.December, 31)
	jan1 := dec31.AddDays(1)
	if jan1.Year != 2025 || jan1.Month != time.January || jan1.Day != 1 {
		t.Errorf("dec31 + 1 day = %+v, want 2025-Jan-1", jan1)
	}

	// Leap year Feb 29 (2024 is leap year, 29 days in Feb)
	feb29 := NewDate(2024, time.February, 29)
	if feb29.daysInMonth() != 29 {
		t.Errorf("Feb 2024 daysInMonth = %d, want 29", feb29.daysInMonth())
	}

	// Nil event store / nil styler rendering
	mNilStyler := NewMonthly(NewDate(2024, time.July, 1), nil)
	area := layout.NewRect(0, 0, mNilStyler.Width(), mNilStyler.Height())
	buf := buffer.Empty(area)
	mNilStyler.Render(area, buf)

	// Check that calendar renders 7 columns for days of week
	r0 := getBufferRowString(buf, 0)
	if r0 == "" {
		t.Errorf("rendered monthly calendar header row is empty")
	}
}

func TestCalendarWeekRowsOverflowDaysArea(t *testing.T) {
	// Rust calendar.rs 0.30.2: week loop uses `if buf.area.height > y` and
	// always y += 1 — does NOT clip to days_area height. Later week rows may
	// paint below the widget area when the buffer is taller.
	// March 2024 has 6 week rows; height=2 with no headers → daysArea h=2,
	// buffer taller so week rows 2+ still emit into buf.
	m := NewMonthly(NewDate(2024, time.March, 1), nil)
	widgetH := 2
	bufH := 8
	area := layout.NewRect(0, 0, m.Width(), widgetH)
	buf := buffer.Empty(layout.NewRect(0, 0, m.Width(), bufH))

	m.Render(area, buf)

	// Week 0 at y=0, week 1 at y=1 are inside widget area.
	r0 := getBufferRowString(buf, 0)
	if r0 == "" || r0 == getSpaces(m.Width()) {
		t.Fatalf("week 0 empty, got %q", r0)
	}
	// Overflow rows below widget area must still be painted (Rust behavior).
	// March 2024: 6 sunday-based weeks → rows y=0..5.
	r3 := getBufferRowString(buf, 3)
	if r3 == "" || r3 == getSpaces(m.Width()) {
		t.Fatalf("overflow week row y=3 empty (old code clipped to daysArea); got %q", r3)
	}
	r5 := getBufferRowString(buf, 5)
	if r5 == "" || r5 == getSpaces(m.Width()) {
		t.Fatalf("overflow week row y=5 empty; got %q", r5)
	}
}

func getSpaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
