package tuikit

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var darkBackground = true

// SetDarkBackground picks the colours for a dark or a light terminal. Call it
// from the TUI on tea.BackgroundColorMsg; an open Modal also follows that
// message by itself.
func SetDarkBackground(dark bool) { darkBackground = dark }

// adaptive is a colour with a shade for each background, chosen at render time.
type adaptive struct{ light, dark string }

func (c adaptive) RGBA() (r, g, b, a uint32) {
	if darkBackground {
		return lipgloss.Color(c.dark).RGBA()
	}
	return lipgloss.Color(c.light).RGBA()
}

var _ color.Color = adaptive{}

var (
	cAccent = adaptive{"#5A56E0", "#8B87FF"}
	cDim    = adaptive{"#7A7A7A", "#8A8A8A"}
	cFaint  = adaptive{"#B8B8B8", "#4A4A4A"}
	cError  = adaptive{"#C0392B", "#FF6B6B"}
	cOK     = adaptive{"#1E8449", "#58D68D"}

	sTitle   = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sLabel   = lipgloss.NewStyle()
	sFocus   = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sFaint   = lipgloss.NewStyle().Foreground(cFaint)
	sError   = lipgloss.NewStyle().Foreground(cError)
	sOK      = lipgloss.NewStyle().Foreground(cOK)
	sBold    = lipgloss.NewStyle().Bold(true)
	sChosen  = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sReverse = lipgloss.NewStyle().Reverse(true).Bold(true)
	sBox     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent).Padding(0, 1)
)
