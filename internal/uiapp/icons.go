package uiapp

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func svgIcon(body string) *ui.SVG {
	src := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">%s</svg>`, body)
	return ui.MustParseSVG([]byte(src))
}

var (
	IconMove     = svgIcon(`<path d="M5 4h.1M10 4h.1M5 8h.1M10 8h.1M5 12h.1M10 12h.1"/>`)
	IconList     = svgIcon(`<path d="M6 4h7M6 8h7M6 12h7M2.5 4h.1M2.5 8h.1M2.5 12h.1"/>`)
	IconTimer    = svgIcon(`<circle cx="8" cy="9" r="5.5"/><path d="M6 1h4M8 6v3l2 1M12 3l1 1"/>`)
	IconCalendar = svgIcon(`<rect x="2" y="3" width="12" height="11" rx="2"/><path d="M5 1.5v3M11 1.5v3M2 7h12M5 10h1M9 10h1"/>`)
	IconSettings = svgIcon(`<path d="M3 2v12M8 2v12M13 2v12M1 5h4M6 11h4M11 6h4"/>`)
	IconPin      = svgIcon(`<path d="M5 2h6l-1 5 3 3H3l3-3-1-5ZM8 10v4"/>`)
	IconClose    = svgIcon(`<path d="M4 4l8 8M12 4l-8 8"/>`)
	IconLeft     = svgIcon(`<path d="M10 3 L5 8 L10 13"/>`)
	IconRight    = svgIcon(`<path d="M6 3 L11 8 L6 13"/>`)
	IconDown     = svgIcon(`<path d="M3 6 L8 11 L13 6"/>`)
	IconCheck    = svgIcon(`<path d="M3 8.5 L6.5 12 L13 4.5"/>`)
	IconAdd      = svgIcon(`<path d="M8 3 V13 M3 8 H13"/>`)
	IconEdit     = svgIcon(`<path d="M11 2.5 L13.5 5 L5.5 13 H3 V10.5 Z"/>`)
	IconDelete   = svgIcon(`<path d="M2.5 4.5 H13.5 M5.5 4.5 V3.2 a1 1 0 0 1 1-1 h3 a1 1 0 0 1 1 1 V4.5 M4 4.5 L4.7 13 a1.5 1.5 0 0 0 1.5 1.4 h3.6 a1.5 1.5 0 0 0 1.5-1.4 L12 4.5"/>`)
	IconAlarm    = svgIcon(`<circle cx="8" cy="8" r="5.8"/><path d="M8 5.2 V8 l2.1 1.2 M6 1.8 v1 M10 1.8 v1"/>`)
	IconMin      = svgIcon(`<path d="M2 8 H14"/>`)
	IconMax      = svgIcon(`<rect x="2.5" y="2.5" width="11" height="11" rx="1.5"/>`)
	IconRestore  = svgIcon(`<rect x="2" y="5" width="9" height="9" rx="1"/><path d="M5 5 V3 a1 1 0 0 1 1-1 h7 a1 1 0 0 1 1 1 v7 a1 1 0 0 1-1 1 h-2"/>`)
	IconSearch   = svgIcon(`<circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5 L14 14"/>`)
	IconTimeline = svgIcon(`<rect x="2" y="2.5" width="12" height="11" rx="2"/><path d="M9.5 2.5 V13.5 M5 6h2.2M5 9.5h2.2"/>`)
	IconCompact  = svgIcon(`<path d="M10 3 H13 V6 M6 13 H3 V10 M13 3 L9.5 6.5 M3 13 L6.5 9.5"/>`)
	IconExpand   = svgIcon(`<path d="M9.5 3 H13 V6.5 M6.5 13 H3 V9.5 M13 3 L9 7 M3 13 L7 9"/>`)
	IconPlay     = svgIcon(`<path d="M5 3.5 L12.5 8 L5 12.5 Z"/>`)
	IconPause    = svgIcon(`<path d="M5.5 3.5 V12.5 M10.5 3.5 V12.5"/>`)
	IconChart    = svgIcon(`<path d="M3 13 V8 M8 13 V4 M13 13 V6.5"/>`)
	IconSidebar  = svgIcon(`<rect x="2" y="2.5" width="12" height="11" rx="2"/><path d="M6 2.5 V13.5"/>`)
	IconPiP      = svgIcon(`<rect x="2" y="3" width="12" height="10" rx="2"/><rect x="7.5" y="7" width="4.5" height="3.8" rx="1"/>`)
	IconScene    = svgIcon(`<path d="M4.5 11.5 H11.8 a2.7 2.7 0 0 0 .3-5.38 A4 4 0 0 0 4.3 5.2 a2.8 2.8 0 0 0 .2 6.3 Z"/><path d="M2.5 14 c1.4-.9 2.6-.9 4 0 s2.6.9 4 0 2.6-.9 3.5-.2"/>`)
)
