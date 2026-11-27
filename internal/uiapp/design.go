package uiapp

import "github.com/egoist/mygo/ui"

// 526Hz design rules. Today is the reference page; every other page follows what it does.
//
//   - Colour: paper white with one ocean-blue ramp. Amber marks an interrupted round,
//     red a destructive action.
//   - The sea is made of glyphs alone (~ ≈ ≋), filled continuously on one flat background,
//     and it stays where it belongs: the Focus Clock, and the tide ribbon on Today.
//   - Every page opens with its title at the same place. Today and the Focus Clock, the two
//     pages the app is about, carry the two-tone title; the other pages use a plain one.
//   - Cards share one radius and one shadow; there is one primary button.
//   - Charts are drawn with the sea's glyphs as columns of water, not with filled bars.
// Type scale. Text is set at one of these sizes and nothing in between:
//
//	11  labels, badges, chart ticks        14  controls and emphasised lines
//	12  metadata and secondary lines       16  a task line, a section or dialog title
//	13  body text and buttons              18  a featured heading
//
// Page titles are 24 (26 for the two-tone ones); large numerals are sized by their own layout.
const (
	PagePadTop  = 18 // space between the title bar and a page's first element
	PagePadX    = 30 // a page's side gutters
	PageTitleFs = 26 // page title size, as on Today
	CardRadius  = 16 // every card
)

// pageTitle is the two-tone heading reserved for Today and the Focus Clock: heavy navy, with the
// last character in ocean blue.
func pageTitle(c *ui.Context, head, tail string, p Palette) ui.Element {
	return ui.RichText(c,
		ui.Span{Text: head, Weight: 800, Size: PageTitleFs, Color: p.Navy},
		ui.Span{Text: tail, Weight: 800, Size: PageTitleFs, Color: p.Blue},
	).LetterSpacing(-0.5)
}

// plainTitle is the heading of the supporting pages (Calendar, Stats, Settings): one colour, no accent.
func plainTitle(c *ui.Context, text string, p Palette) ui.Element {
	return ui.Text(c, text).FontSize(24).FontWeight(700).TextColor(p.Navy).SingleLine()
}

// cardLook applies the one card style (Today's header card) to el.
func cardLook(el ui.Element, p Palette) ui.Element {
	return el.Radius(CardRadius).Background(p.Paper).Border(1, p.Line).Shadow(0, 4, 18, 0, p.CardShadow)
}

// primaryBtn is the one primary button: Today's ocean-blue button, at any text size.
// Callers chain Height / Padding / Radius as they would on a stock button.
func primaryBtn(c *ui.Context, label string, fs float32, p Palette) ui.Element {
	b := ui.ButtonBase(c).Height(32).Padding(0, 14).Radius(9).Center().
		LinearGradient(ui.LinearGradient{From: p.Blue, To: p.BlueDeep, Angle: 180, Oklab: true}).
		Shadow(0, 3, 10, 0, p.BtnGlow).
		Transition(cardFadeFast).Cursor(ui.CursorPointer).Label(label)
	if b.Hovered() {
		b.LinearGradient(ui.LinearGradient{From: p.Blue2, To: p.Blue, Angle: 180, Oklab: true}).
			Shadow(0, 5, 14, 0, p.BtnGlow)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(fs).FontWeight(700).TextColor(ui.Hex("#FFFFFF")).SingleLine()
	})
	return b
}
