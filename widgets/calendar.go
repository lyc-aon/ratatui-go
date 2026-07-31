package widgets

import (
	"fmt"
	"time"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/text"
)

// Date is a calendar day without timezone (year/month/day only).
//
// Avoids time.Time timezone drift; arithmetic uses civil date math.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate builds a Date. Month/day are not validated beyond Go's time rules
// when converted via time.Date for weekday/month-length helpers.
func NewDate(year int, month time.Month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

// TodayDate returns today's local civil date.
func TodayDate() Date {
	now := time.Now()
	y, m, d := now.Date()
	return Date{Year: y, Month: m, Day: d}
}

// Equal reports whether two dates are the same civil day.
func (d Date) Equal(other Date) bool {
	return d.Year == other.Year && d.Month == other.Month && d.Day == other.Day
}

// timeDate converts to a UTC time.Time at midnight for weekday / month length.
// UTC avoids local DST shifting the civil day.
func (d Date) timeDate() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// Weekday returns the day of week for d.
func (d Date) Weekday() time.Weekday {
	return d.timeDate().Weekday()
}

// AddDays returns d advanced by n days (n may be negative).
func (d Date) AddDays(n int) Date {
	t := d.timeDate().AddDate(0, 0, n)
	y, m, day := t.Date()
	return Date{Year: y, Month: m, Day: day}
}

// daysInMonth returns the number of days in d's month.
func (d Date) daysInMonth() int {
	// Day 0 of next month = last day of this month.
	t := time.Date(d.Year, d.Month+1, 0, 0, 0, 0, 0, time.UTC)
	return t.Day()
}

// firstOfMonth returns the 1st of d's month.
func (d Date) firstOfMonth() Date {
	return Date{Year: d.Year, Month: d.Month, Day: 1}
}

// lastOfMonth returns the last day of d's month.
func (d Date) lastOfMonth() Date {
	return Date{Year: d.Year, Month: d.Month, Day: d.daysInMonth()}
}

// sundayBasedWeek is a Sunday-origin week index used only for differences.
//
// It is difference-stable within a year (last-first+1 matches time crate
// sunday_based_week spans used by Height), but is NOT absolutely equal to
// time::Date::sunday_based_week — years where Jan 1 is Sunday differ by 1.
func (d Date) sundayBasedWeek() int {
	t := d.timeDate()
	// Ordinal day shifted by Jan 1's weekday-from-Sunday, then /7.
	ord := t.YearDay() // 1..366
	jan1 := time.Date(d.Year, time.January, 1, 0, 0, 0, 0, time.UTC)
	// Go Weekday: Sunday=0 ... Saturday=6 (same as number_days_from_sunday).
	fromSun := int(jan1.Weekday())
	return (ord - 1 + fromSun) / 7
}

// DateStyler provides a style for a given date.
type DateStyler interface {
	GetStyle(date Date) style.Style
}

// CalendarEventStore is a map-backed DateStyler (last write wins).
type CalendarEventStore struct {
	events map[Date]style.Style
}

// NewCalendarEventStore creates an empty store.
func NewCalendarEventStore() CalendarEventStore {
	return CalendarEventStore{events: make(map[Date]style.Style, 4)}
}

// Today builds a store that styles today's local date.
func Today(st style.Style) CalendarEventStore {
	s := NewCalendarEventStore()
	s.Add(TodayDate(), st)
	return s
}

// Add records a style for date. Last write wins.
func (s *CalendarEventStore) Add(date Date, st style.Style) {
	if s.events == nil {
		s.events = make(map[Date]style.Style, 4)
	}
	s.events[date] = st
}

// GetStyle implements DateStyler. Missing dates return the zero Style.
func (s CalendarEventStore) GetStyle(date Date) style.Style {
	if s.events == nil {
		return style.Style{}
	}
	if st, ok := s.events[date]; ok {
		return st
	}
	return style.Style{}
}

// emptyDateStyler returns default style for every date.
type emptyDateStyler struct{}

func (emptyDateStyler) GetStyle(Date) style.Style { return style.Style{} }

// Monthly renders a month calendar for the month containing DisplayDate.
//
// Grid is Sunday-first. Day cells are 2 columns with a 1-column gutter
// (width 7*(1+2)=21). Surrounding-month days show only when ShowSurrounding
// is set. Event styles from the DateStyler patch over default/surrounding.
type Monthly struct {
	displayDate     Date
	events          DateStyler
	showSurrounding *style.Style
	showWeekday     *style.Style
	showMonth       *style.Style
	defaultStyle    style.Style
	block           *Block
}

// NewMonthly constructs a calendar for displayDate using styler for events.
// A nil styler is treated as an empty store (always default style).
func NewMonthly(displayDate Date, styler DateStyler) Monthly {
	if styler == nil {
		styler = emptyDateStyler{}
	}
	return Monthly{
		displayDate:  displayDate,
		events:       styler,
		defaultStyle: style.New(),
	}
}

// ShowSurrounding fills slots for days outside the display month with style.
// Event styles still patch on top.
func (m Monthly) ShowSurrounding(st style.Style) Monthly {
	m.showSurrounding = &st
	return m
}

// ShowWeekdaysHeader shows " Su Mo Tu We Th Fr Sa" with style.
func (m Monthly) ShowWeekdaysHeader(st style.Style) Monthly {
	m.showWeekday = &st
	return m
}

// ShowMonthHeader shows a centered "Month Year" header with style.
func (m Monthly) ShowMonthHeader(st style.Style) Monthly {
	m.showMonth = &st
	return m
}

// DefaultStyle sets the style for in-month days without event styles.
func (m Monthly) DefaultStyle(st style.Style) Monthly {
	m.defaultStyle = st
	return m
}

// Block surrounds the calendar with a block.
func (m Monthly) Block(b Block) Monthly {
	m.block = &b
	return m
}

// Width returns cells required to render the calendar (grid + optional block).
//
// Base grid is 21 (7 days × (1 gutter + 2 day cells)).
func (m Monthly) Width() int {
	const (
		daysPerWeek = 7
		gutterWidth = 1
		dayWidth    = 2
	)
	width := daysPerWeek * (gutterWidth + dayWidth)
	if m.block != nil {
		left, right := m.block.HorizontalSpace()
		width = satAdd(width, satAdd(left, right))
	}
	return width
}

// Height returns rows required (week rows + optional headers + block chrome).
func (m Monthly) Height() int {
	height := sundayBasedWeeks(m.displayDate)
	if m.showMonth != nil {
		height = satAdd(height, 1)
	}
	if m.showWeekday != nil {
		height = satAdd(height, 1)
	}
	if m.block != nil {
		top, bottom := m.block.VerticalSpace()
		height = satAdd(height, satAdd(top, bottom))
	}
	return height
}

// Render draws the monthly calendar into buf within area.
//
// Intersects with buf.Area; empty/zero never panics.
func (m Monthly) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	// Upstream Monthly never base-fills the widget area; defaultStyle only
	// reaches day spans via formatDate / defaultBG gutters.
	inner := InnerIfSome(m.block, area, buf)
	m.renderMonthly(inner, buf)
}

func (m Monthly) renderMonthly(area layout.Rect, buf *buffer.Buffer) {
	if area.IsEmpty() || buf == nil {
		return
	}

	monthH, weekH := 0, 0
	if m.showMonth != nil {
		monthH = 1
	}
	if m.showWeekday != nil {
		weekH = 1
	}
	parts := layout.Vertical(
		layout.Length(monthH),
		layout.Length(weekH),
		layout.Fill(1),
	).Split(area)
	var monthHeader, daysHeader, daysArea layout.Rect
	if len(parts) >= 3 {
		monthHeader, daysHeader, daysArea = parts[0], parts[1], parts[2]
	} else {
		daysArea = area
	}

	if m.showMonth != nil && !monthHeader.IsEmpty() {
		// Line::render fills the whole row with the line style before glyphs.
		buf.SetStyle(monthHeader.Intersection(buf.Area), *m.showMonth)
		line := text.StyledLine(
			fmt.Sprintf("%s %d", m.displayDate.Month.String(), m.displayDate.Year),
			*m.showMonth,
		).Centered()
		renderLineInArea(buf, monthHeader, line)
	}

	if m.showWeekday != nil && !daysHeader.IsEmpty() {
		line := text.StyledLine(" Su Mo Tu We Th Fr Sa", *m.showWeekday)
		renderLineInArea(buf, daysHeader, line)
	}

	if daysArea.IsEmpty() {
		return
	}

	// Start at the Sunday on-or-before the 1st of the month.
	first := m.displayDate.firstOfMonth()
	offset := int(first.Weekday()) // Sunday=0 in Go
	curr := first.AddDays(-offset)

	// Walk weeks until we leave the display month into the *next* month and
	// finish that week's row start — matches upstream:
	// while curr_day.month() != display.month().next()
	nextMonth := m.displayDate.Month + 1
	nextYear := m.displayDate.Year
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}

	y := daysArea.Y
	for curr.Year != nextYear || curr.Month != nextMonth {
		// Rust calendar.rs: emit while buf.area.height > y — do not clip to
		// daysArea height, so later week rows may overflow the widget area.
		// SetLine/SetStringN already no-op safely outside the buffer.
		spans := make([]text.Span, 0, 14)
		for i := range 7 {
			if i == 0 {
				spans = append(spans, text.StyledSpan(" ", style.New()))
			} else {
				spans = append(spans, text.StyledSpan(" ", m.defaultBG()))
			}
			spans = append(spans, m.formatDate(curr))
			curr = curr.AddDays(1)
		}
		if buf.Area.Height > y {
			line := text.FromSpanSlice(spans)
			// Upstream set_line uses the full monthly area width as the clip.
			buf.SetLine(daysArea.X, y, line, area.Width)
		}
		y++
	}
}

// defaultBG is a style carrying only the default background color.
func (m Monthly) defaultBG() style.Style {
	if !m.defaultStyle.HasBG {
		return style.New()
	}
	return style.New().WithBG(m.defaultStyle.BG)
}

// formatDate builds the 2-wide day span with style precedence:
// in-month: default.patch(event)
// surrounding shown: default.patch(surrounding).patch(event)
// surrounding hidden: two spaces with default background only.
func (m Monthly) formatDate(date Date) text.Span {
	dayStr := fmt.Sprintf("%2d", date.Day)
	if date.Month == m.displayDate.Month && date.Year == m.displayDate.Year {
		st := m.defaultStyle.Patch(m.events.GetStyle(date))
		return text.StyledSpan(dayStr, st)
	}
	if m.showSurrounding == nil {
		return text.StyledSpan("  ", m.defaultBG())
	}
	st := m.defaultStyle.Patch(*m.showSurrounding).Patch(m.events.GetStyle(date))
	return text.StyledSpan(dayStr, st)
}

// sundayBasedWeeks is how many Sunday-first week rows the month needs.
func sundayBasedWeeks(display Date) int {
	first := display.firstOfMonth()
	last := display.lastOfMonth()
	firstWeek := first.sundayBasedWeek()
	lastWeek := last.sundayBasedWeek()
	if lastWeek < firstWeek {
		// Year wrap shouldn't happen within one month, but saturate safely.
		return 1
	}
	return lastWeek - firstWeek + 1
}
