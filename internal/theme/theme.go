package theme

import "image/color"

// Theme defines the color scheme for the application. Colors always come
// from the active Omarchy theme; see omarchy.go.
type Theme struct {
	Name string

	// Text colors
	Primary   color.Color
	Secondary color.Color
	Accent    color.Color
	Muted     color.Color
	Error     color.Color
	Success   color.Color
	Warning   color.Color

	// UI element colors
	Border       color.Color
	BorderActive color.Color
	Background   color.Color
	Highlight    color.Color
	Shadow       color.Color
}
