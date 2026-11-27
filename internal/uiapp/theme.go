package uiapp

import (
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	FontSans = "Segoe UI Variable Text, Segoe UI, Microsoft YaHei UI, Microsoft YaHei, sans-serif"
	FontNum  = "Bahnschrift SemiBold, Bahnschrift, Segoe UI Variable Display, Microsoft YaHei UI, sans-serif"
	FontMono = "Cascadia Mono, Consolas, monospace"
)

// Palette mirrors studio.css semantic tokens for both light and dark modes.
type Palette struct {
	Dark         bool
	Paper        ui.Color
	Surface      ui.Color
	Surface2     ui.Color
	Blue         ui.Color
	Blue2        ui.Color
	BlueSoft     ui.Color
	BlueDeep     ui.Color
	Navy         ui.Color
	Ink          ui.Color
	Ink2         ui.Color
	Ink3         ui.Color
	Ink4         ui.Color
	Success      ui.Color
	SuccessSoft  ui.Color
	SuccessLine  ui.Color
	DoneBadgeBg  ui.Color
	DoneBadgeInk ui.Color
	AccentSoft   ui.Color
	AccentVeil   ui.Color
	Warn         ui.Color
	WarnBg       ui.Color
	WarnLine     ui.Color
	WarnInk      ui.Color
	Danger       ui.Color
	DangerSoft   ui.Color
	DangerLine   ui.Color
	RolledBg     ui.Color
	RolledInk    ui.Color
	CreditBg     ui.Color
	Run          ui.Color
	Line         ui.Color
	LineSoft     ui.Color
	ProgressBg   ui.Color
	CheckBorder  ui.Color
	Strike       ui.Color
	Scrollbar    ui.Color
	IdleDot      ui.Color
	HoverTint    ui.Color
	BorderSubtle ui.Color
	BorderMid    ui.Color
	ShellBg      ui.Color
	TitleText    ui.Color

	// Timeline & Chart
	TlBg          ui.Color
	TlGrid        ui.Color
	TlGridMajor   ui.Color
	TlNight       ui.Color
	TlBlockBg     ui.Color
	TlBlockBorder ui.Color
	TlWarnBg      ui.Color
	TlWarnBorder  ui.Color
	TlNow         ui.Color
	StatHeroA     ui.Color
	StatHeroB     ui.Color
	ChartBgA      ui.Color
	ChartBgB      ui.Color
	ChartGrid     ui.Color
	ChartSlot     ui.Color

	// Timer surfaces
	TimerDockBg     ui.Color
	TimerChipBg     ui.Color
	TimerSegBg      ui.Color
	TimerDoneBg     ui.Color
	TimerDoneDetail ui.Color
	TimerVeilTop    ui.Color
	TimerVeilBot    ui.Color
	TimerVeilZero   ui.Color
	TimerNumGlow    ui.Color
	TimerHCBack     ui.Color
	ModalBackdrop   ui.Color

	// Same-hue elevation, specular sheen & tactile shadow tokens
	CardShadow     ui.Color
	ElevatedShadow ui.Color
	TopRimLight    ui.Color
	BtnGlow        ui.Color
}

var LightPalette = Palette{
	Dark:            false,
	Paper:           ui.Hex("#FFFFFF"),
	Surface:         ui.Hex("#F3F7FC"),
	Surface2:        ui.Hex("#F8FAFD"),
	Blue:            ui.Hex("#007CFF"),
	Blue2:           ui.Hex("#00A1FF"),
	BlueSoft:        ui.Hex("#91D4FF"),
	BlueDeep:        ui.Hex("#0066D6"),
	Navy:            ui.Hex("#002E58"),
	Ink:             ui.Hex("#122D46"),
	Ink2:            ui.Hex("#465E73"),
	Ink3:            ui.Hex("#60778B"),
	Ink4:            ui.Hex("#708497"),
	Success:         ui.Hex("#0B6B4F"),
	SuccessSoft:     ui.Hex("#EAF7F0"),
	SuccessLine:     ui.Hex("#B9DFCF"),
	DoneBadgeBg:     ui.Hex("#E7F5EB"),
	DoneBadgeInk:    ui.Hex("#1E8E4E"),
	AccentSoft:      ui.Hex("#EDF6FF"),
	AccentVeil:      ui.Hex("#C6E2FF"),
	Warn:            ui.Hex("#E08A00"),
	WarnBg:          ui.Hex("#FFF4E0"),
	WarnLine:        ui.Hex("#E7CEA5"),
	WarnInk:         ui.Hex("#7A4A00"),
	Danger:          ui.Hex("#A4262C"),
	DangerSoft:      ui.Hex("#FDF2F2"),
	DangerLine:      ui.Hex("#E8C6C9"),
	RolledBg:        ui.Hex("#FBF1E3"),
	RolledInk:       ui.Hex("#9A5B14"),
	CreditBg:        ui.Hex("#EEF3F8"),
	Run:             ui.Hex("#0FA958"),
	Line:            ui.Hex("#DCE6EF"),
	LineSoft:        ui.Hex("#E8EEF4"),
	ProgressBg:      ui.Hex("#E6EFF7"),
	CheckBorder:     ui.Hex("#A5BACB"),
	Strike:          ui.Hex("#B5C5D3"),
	Scrollbar:       ui.Hex("#B6CADA"),
	IdleDot:         ui.Hex("#8BA4B8"),
	HoverTint:       ui.RGBA(0, 46, 88, 0.08),
	BorderSubtle:    ui.RGBA(0, 46, 88, 0.10),
	BorderMid:       ui.RGBA(0, 46, 88, 0.16),
	ShellBg:         ui.Hex("#F3F7FC"),
	TitleText:       ui.Hex("#002E58"),
	TlBg:            ui.Hex("#F1F6FC"),
	TlGrid:          ui.RGBA(0, 46, 88, 0.045),
	TlGridMajor:     ui.RGBA(0, 46, 88, 0.09),
	TlNight:         ui.RGBA(0, 46, 88, 0.026),
	TlBlockBg:       ui.RGBA(255, 255, 255, 0.94),
	TlBlockBorder:   ui.Hex("#DCE9F6"),
	TlWarnBg:        ui.Hex("#FFFBF3"),
	TlWarnBorder:    ui.Hex("#F0DCC0"),
	TlNow:           ui.Hex("#E05252"),
	StatHeroA:       ui.Hex("#E9F4FF"),
	StatHeroB:       ui.Hex("#F8FCFF"),
	ChartBgA:        ui.Hex("#FCFDFF"),
	ChartBgB:        ui.Hex("#F5F9FE"),
	ChartGrid:       ui.RGBA(0, 46, 88, 0.06),
	ChartSlot:       ui.RGBA(0, 46, 88, 0.03),
	TimerDockBg:     ui.RGBA(247, 251, 255, 0.96),
	TimerChipBg:     ui.RGBA(255, 255, 255, 0.92),
	TimerSegBg:      ui.RGBA(255, 255, 255, 0.78),
	TimerDoneBg:     ui.RGBA(240, 244, 248, 0.86),
	TimerDoneDetail: ui.Hex("#F4F9FF"),
	TimerVeilTop:    ui.RGBA(243, 247, 252, 0.92),
	TimerVeilBot:    ui.RGBA(243, 247, 252, 0.94),
	TimerVeilZero:   ui.RGBA(243, 247, 252, 0.0),
	TimerNumGlow:    ui.RGBA(248, 252, 255, 0.96),
	TimerHCBack:     ui.RGBA(255, 255, 255, 0.96),
	ModalBackdrop:   ui.RGBA(8, 25, 44, 0.42),
	CardShadow:      ui.RGBA(0, 46, 88, 0.055),
	ElevatedShadow:  ui.RGBA(0, 46, 88, 0.13),
	TopRimLight:     ui.RGBA(255, 255, 255, 0.85),
	BtnGlow:         ui.RGBA(0, 124, 255, 0.24),
}

var DarkPalette = Palette{
	Dark:            true,
	Paper:           ui.Hex("#131C28"),
	Surface:         ui.Hex("#0D141E"),
	Surface2:        ui.Hex("#182332"),
	Blue:            ui.Hex("#3B9BFF"),
	Blue2:           ui.Hex("#62B4FF"),
	BlueSoft:        ui.Hex("#85C7FF"),
	BlueDeep:        ui.Hex("#2584F0"),
	Navy:            ui.Hex("#EBF3FC"),
	Ink:             ui.Hex("#E2ECF7"),
	Ink2:            ui.Hex("#B4C6D8"),
	Ink3:            ui.Hex("#879DB4"),
	Ink4:            ui.Hex("#6D849C"),
	Success:         ui.Hex("#22B573"),
	SuccessSoft:     ui.RGBA(34, 181, 115, 0.14),
	SuccessLine:     ui.RGBA(34, 181, 115, 0.36),
	DoneBadgeBg:     ui.RGBA(34, 181, 115, 0.16),
	DoneBadgeInk:    ui.Hex("#57D89E"),
	AccentSoft:      ui.Hex("#1A2B40"),
	AccentVeil:      ui.Hex("#2B4B72"),
	Warn:            ui.Hex("#F0A32E"),
	WarnBg:          ui.RGBA(240, 163, 46, 0.16),
	WarnLine:        ui.RGBA(240, 163, 46, 0.38),
	WarnInk:         ui.Hex("#F8C56E"),
	Danger:          ui.Hex("#F06A72"),
	DangerSoft:      ui.RGBA(240, 106, 114, 0.14),
	DangerLine:      ui.RGBA(240, 106, 114, 0.38),
	RolledBg:        ui.RGBA(240, 163, 46, 0.16),
	RolledInk:       ui.Hex("#F8C56E"),
	CreditBg:        ui.Hex("#1D2B3B"),
	Run:             ui.Hex("#22C574"),
	Line:            ui.Hex("#243447"),
	LineSoft:        ui.Hex("#1E2C3D"),
	ProgressBg:      ui.Hex("#203042"),
	CheckBorder:     ui.Hex("#4B647F"),
	Strike:          ui.Hex("#586F87"),
	Scrollbar:       ui.Hex("#354A63"),
	IdleDot:         ui.Hex("#6D849C"),
	HoverTint:       ui.RGBA(180, 210, 245, 0.10),
	BorderSubtle:    ui.RGBA(180, 210, 245, 0.10),
	BorderMid:       ui.RGBA(180, 210, 245, 0.18),
	ShellBg:         ui.Hex("#0D141E"),
	TitleText:       ui.Hex("#EBF3FC"),
	TlBg:            ui.Hex("#0F1722"),
	TlGrid:          ui.RGBA(180, 210, 245, 0.06),
	TlGridMajor:     ui.RGBA(180, 210, 245, 0.13),
	TlNight:         ui.RGBA(4, 9, 16, 0.38),
	TlBlockBg:       ui.RGBA(24, 36, 52, 0.94),
	TlBlockBorder:   ui.Hex("#2B425C"),
	TlWarnBg:        ui.RGBA(74, 50, 18, 0.55),
	TlWarnBorder:    ui.RGBA(240, 163, 46, 0.36),
	TlNow:           ui.Hex("#F06262"),
	StatHeroA:       ui.Hex("#162A42"),
	StatHeroB:       ui.Hex("#132134"),
	ChartBgA:        ui.Hex("#15202E"),
	ChartBgB:        ui.Hex("#111A26"),
	ChartGrid:       ui.RGBA(180, 210, 245, 0.08),
	ChartSlot:       ui.RGBA(180, 210, 245, 0.04),
	TimerDockBg:     ui.RGBA(16, 24, 36, 0.95),
	TimerChipBg:     ui.RGBA(24, 35, 50, 0.92),
	TimerSegBg:      ui.RGBA(24, 35, 50, 0.85),
	TimerDoneBg:     ui.RGBA(10, 16, 24, 0.88),
	TimerDoneDetail: ui.Hex("#182536"),
	TimerVeilTop:    ui.RGBA(13, 20, 30, 0.92),
	TimerVeilBot:    ui.RGBA(13, 20, 30, 0.95),
	TimerVeilZero:   ui.RGBA(13, 20, 30, 0.0),
	TimerNumGlow:    ui.RGBA(13, 20, 30, 0.95),
	TimerHCBack:     ui.RGBA(15, 24, 36, 0.95),
	ModalBackdrop:   ui.RGBA(3, 8, 16, 0.68),
	CardShadow:      ui.RGBA(0, 0, 0, 0.34),
	ElevatedShadow:  ui.RGBA(0, 0, 0, 0.52),
	TopRimLight:     ui.RGBA(180, 215, 255, 0.09),
	BtnGlow:         ui.RGBA(59, 155, 255, 0.28),
}

var (
	lightUITheme *ui.Theme
	darkUITheme  *ui.Theme

	nativeThemeMu   sync.Mutex
	lastNativeTheme string
)

func init() {
	lightUITheme = makeUITheme(false)
	darkUITheme = makeUITheme(true)
}

func ResolveDark(themeMode string, sysDark bool) bool {
	switch themeMode {
	case "dark":
		return true
	case "system":
		return sysDark
	default:
		return false
	}
}

func makeUITheme(dark bool) *ui.Theme {
	if dark {
		p := DarkPalette
		t := ui.DarkTheme()
		t.Dark = true
		t.Background = p.Paper
		t.Surface = p.Surface2
		t.SurfaceHover = ui.Hex("#202E41")
		t.SurfacePressed = ui.Hex("#283950")
		t.Border = p.Line
		t.Text = p.Ink
		t.TextMuted = p.Ink3
		t.Accent = p.Blue
		t.AccentHover = p.Blue2
		t.AccentPressed = p.BlueDeep
		t.AccentText = ui.Hex("#FFFFFF")
		t.InactiveAccent = p.Blue
		t.Danger = p.Danger
		t.Warning = p.Warn
		t.Success = p.Success
		t.Selection = ui.Hex("#1F4B7A")
		t.Focus = p.Blue.Alpha(0.65)
		t.Scrollbar = p.Scrollbar
		t.ScrollbarWidth = 6
		t.Radius = 10
		t.Spacing = 4
		t.Font = FontSans
		t.FontSize = 14
		return t
	}
	p := LightPalette
	t := ui.LightTheme()
	t.Dark = false
	t.Background = p.Paper
	t.Surface = p.Surface
	t.SurfaceHover = p.AccentSoft
	t.SurfacePressed = ui.Hex("#DCEBFB")
	t.Border = p.Line
	t.Text = p.Ink
	t.TextMuted = p.Ink3
	t.Accent = p.BlueDeep
	t.AccentHover = p.Blue
	t.AccentPressed = p.Navy
	t.AccentText = ui.Hex("#FFFFFF")
	t.InactiveAccent = p.BlueDeep
	t.Danger = p.Danger
	t.Warning = p.Warn
	t.Success = p.Success
	t.Selection = ui.Hex("#CFE7FF")
	t.Focus = p.Blue.Alpha(0.6)
	t.Scrollbar = p.Scrollbar
	t.ScrollbarWidth = 6
	t.Radius = 10
	t.Spacing = 4
	t.Font = FontSans
	t.FontSize = 14
	return t
}

// BuildUITheme returns a cached immutable *ui.Theme and Palette with zero per-frame allocations.
func BuildUITheme(dark bool) (*ui.Theme, Palette) {
	if dark {
		return darkUITheme, DarkPalette
	}
	return lightUITheme, LightPalette
}

func SyncNativeThemeMode(mode string) {
	if !mygo.App.IsReady() {
		return
	}
	nativeThemeMu.Lock()
	if mode == lastNativeTheme {
		nativeThemeMu.Unlock()
		return
	}
	lastNativeTheme = mode
	nativeThemeMu.Unlock()

	switch mode {
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	case "system":
		mygo.Theme.SetSource(mygo.ThemeSystem)
	default:
		mygo.Theme.SetSource(mygo.ThemeLight)
	}
}
