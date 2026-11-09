// Package theme holds the built-in color themes: the terminal palette plus the window chrome colors.
package theme

import (
	"image/color"

	"github.com/beefysalad/kaze-terminal/internal/render"
)

// Chrome is every color the window chrome (title bar, sidebar, status bar, toasts) draws with.
type Chrome struct {
	Chrome, Active, ActiveFG, Muted, Border, Accent, StatusFG, Folder, Branch color.NRGBA
	CapGlyph, CapHover, CapPress, CloseHover, ClosePress                      color.NRGBA
	Needs, Done, ToastBG, ToastLine, ToastFG, ToastGo, ToastTitle, UsageHigh  color.NRGBA
	// An edit's added and removed lines in the readable agent activity.
	DiffAddBG, DiffAddFG, DiffDelBG, DiffDelFG color.NRGBA
	// Token colours for code shown in the readable agent activity.
	CodeKeyword, CodeString, CodeComment, CodeNumber, CodeName color.NRGBA
}

// Theme is one selectable look.
type Theme struct {
	Key, Name string
	Term      render.Palette
	UI        Chrome
}

func hex(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func ansi(v ...uint32) (a [16]color.NRGBA) {
	for i, c := range v {
		a[i] = hex(c)
	}
	return a
}

// All returns the built-in themes in picker order.
func All() []Theme {
	return []Theme{midnight(), sumi(), tide(), washi(), sakura(), matcha(), koyo(), hai(), fuji(), kiri()}
}

// ByKey finds a theme by its config key.
func ByKey(k string) (Theme, bool) {
	for _, t := range All() {
		if t.Key == k {
			return t, true
		}
	}
	return Theme{}, false
}

// Default is Midnight, the original look.
func Default() Theme { return midnight() }

// Close-button reds are shared: a red hover reads the same on every theme.
var (
	closeHover = hex(0xe5484d)
	closePress = hex(0xc93c40)
)

func midnight() Theme {
	return Theme{
		Key: "midnight", Name: "Midnight", Term: render.DefaultPalette(),
		UI: Chrome{
			Chrome: hex(0x0b0d12), Active: hex(0x0f1117), ActiveFG: hex(0xffffff), Muted: hex(0x8a90a2),
			Border: hex(0x1d2230), Accent: hex(0x8b7bff), StatusFG: hex(0x7d8394), Folder: hex(0xc9cdd8),
			Branch: hex(0x62d39a), CapGlyph: hex(0xa7adbd), CapHover: hex(0x1a1f2c), CapPress: hex(0x151925),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0xf2b84b), Done: hex(0x62d39a),
			ToastBG: hex(0x1a1f2c), ToastLine: hex(0x2a3142), ToastFG: hex(0xe7e9f0), ToastGo: hex(0xb9a6ff),
			ToastTitle: hex(0xffffff), UsageHigh: hex(0xff6b6b),
			DiffAddBG: hex(0x122419), DiffAddFG: hex(0x8fe0b0), DiffDelBG: hex(0x2a1418), DiffDelFG: hex(0xf3a0a8),
			CodeKeyword: hex(0xb9a6ff), CodeString: hex(0x8fe0b0), CodeComment: hex(0x8a90a2),
			CodeNumber: hex(0xf2b84b), CodeName: hex(0x8fc8ff),
		},
	}
}

func sumi() Theme {
	return Theme{
		Key: "sumi", Name: "Sumi",
		Term: render.Palette{
			FG: hex(0xe4e1da), BG: hex(0x111111), Cursor: hex(0xe2553d), Selection: hex(0x3a2420),
			ANSI: ansi(0x1c1c1c, 0xe2553d, 0x9fbf8f, 0xd9b56a, 0x7f9fbf, 0xc08fa8, 0x8fbfb5, 0xe4e1da,
				0x55524c, 0xff7a5c, 0xb8d6a8, 0xf0cf86, 0xa3bfdc, 0xd9acc2, 0xa9d9cf, 0xf3efe6),
			Match: hex(0x2a3a5c), MatchCur: hex(0xd9b56a), MatchMark: hex(0x7f8fb8),
		},
		UI: Chrome{
			Chrome: hex(0x0c0c0c), Active: hex(0x111111), ActiveFG: hex(0xffffff), Muted: hex(0x77736b),
			Border: hex(0x222222), Accent: hex(0xe2553d), StatusFG: hex(0x77736b), Folder: hex(0xe4e1da),
			Branch: hex(0x9fbf8f), CapGlyph: hex(0x77736b), CapHover: hex(0x2e2e2e), CapPress: hex(0x191919),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0xd9b56a), Done: hex(0x9fbf8f),
			ToastBG: hex(0x181818), ToastLine: hex(0x222222), ToastFG: hex(0xe4e1da), ToastGo: hex(0xe2553d),
			ToastTitle: hex(0xffffff), UsageHigh: hex(0xff7a5c),
			DiffAddBG: hex(0x16261a), DiffAddFG: hex(0xa9d79a), DiffDelBG: hex(0x2c1513), DiffDelFG: hex(0xf0a093),
			CodeKeyword: hex(0xe08a78), CodeString: hex(0xa9d79a), CodeComment: hex(0x8f8b83),
			CodeNumber: hex(0xd9b56a), CodeName: hex(0x9fbfdc),
		},
	}
}

func tide() Theme {
	return Theme{
		Key: "tide", Name: "Tide",
		Term: render.Palette{
			FG: hex(0xcfe3ef), BG: hex(0x0b1626), Cursor: hex(0x40e0d0), Selection: hex(0x123a48),
			ANSI: ansi(0x132235, 0xff7b8a, 0x5fe0a8, 0xffd27a, 0x7cc8ff, 0xc3a6ff, 0x5fe0d0, 0xcfe3ef,
				0x3f5b6b, 0xff9daa, 0x8ff0c2, 0xffe29f, 0xa6dcff, 0xd8c4ff, 0x8ff0e4, 0xe9fbff),
			Match: hex(0x42316f), MatchCur: hex(0xffd27a), MatchMark: hex(0x7c86d8),
		},
		UI: Chrome{
			Chrome: hex(0x08111e), Active: hex(0x0b1626), ActiveFG: hex(0xffffff), Muted: hex(0x7fa3b8),
			Border: hex(0x16283d), Accent: hex(0x40e0d0), StatusFG: hex(0x6e8ea2), Folder: hex(0xcfe3ef),
			Branch: hex(0x5fe0a8), CapGlyph: hex(0x7fa3b8), CapHover: hex(0x1e3550), CapPress: hex(0x101e2f),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0xffd27a), Done: hex(0x5fe0a8),
			ToastBG: hex(0x0c1e30), ToastLine: hex(0x16283d), ToastFG: hex(0xcfe3ef), ToastGo: hex(0x40e0d0),
			ToastTitle: hex(0xffffff), UsageHigh: hex(0xff9daa),
			DiffAddBG: hex(0x12292d), DiffAddFG: hex(0x8ff0c2), DiffDelBG: hex(0x2a1c2a), DiffDelFG: hex(0xffb3bd),
			CodeKeyword: hex(0xc3a6ff), CodeString: hex(0x8ff0c2), CodeComment: hex(0x7fa3b8),
			CodeNumber: hex(0xffd27a), CodeName: hex(0x7cc8ff),
		},
	}
}

// Washi is the light theme, so its caption hover/press go darker, not lighter.
func washi() Theme {
	return Theme{
		Key: "washi", Name: "Washi",
		Term: render.Palette{
			FG: hex(0x2b2a33), BG: hex(0xf6f2ea), Cursor: hex(0x3f51b5), Selection: hex(0xd6dcf5),
			ANSI: ansi(0x2b2a33, 0xc0392b, 0x2e7d4f, 0x9a6a00, 0x2f5fb3, 0x8e44ad, 0x1f7f8a, 0xbdb6a8,
				0x6b665c, 0xd9534a, 0x3c9a63, 0xb8860b, 0x4a7bd0, 0xa35fc2, 0x2a9aa6, 0x1a1a1f),
			Match: hex(0xefe0a6), MatchCur: hex(0x9a6a00), MatchMark: hex(0x7d5fb0),
		},
		UI: Chrome{
			Chrome: hex(0xece6da), Active: hex(0xf6f2ea), ActiveFG: hex(0x2b2a33), Muted: hex(0x7a7468),
			Border: hex(0xddd5c6), Accent: hex(0x3f51b5), StatusFG: hex(0x8a8478), Folder: hex(0x2b2a33),
			Branch: hex(0x2e7d4f), CapGlyph: hex(0x7a7468), CapHover: hex(0xcfc7b7), CapPress: hex(0xc4bcab),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0x9a6a00), Done: hex(0x2e7d4f),
			ToastBG: hex(0xfffdf8), ToastLine: hex(0xddd5c6), ToastFG: hex(0x2b2a33), ToastGo: hex(0x3f51b5),
			ToastTitle: hex(0x2b2a33), UsageHigh: hex(0xc0392b),
			DiffAddBG: hex(0xd7dfcc), DiffAddFG: hex(0x1f5a37), DiffDelBG: hex(0xedd3c9), DiffDelFG: hex(0x962d22),
			CodeKeyword: hex(0x6a3a9a), CodeString: hex(0x1f5a37), CodeComment: hex(0x565c50),
			CodeNumber: hex(0x7a4f00), CodeName: hex(0x1f4f9a),
		},
	}
}

func sakura() Theme {
	return Theme{
		Key: "sakura", Name: "Sakura",
		Term: render.Palette{
			FG: hex(0xeedde6), BG: hex(0x1a1218), Cursor: hex(0xff8fb8), Selection: hex(0x4a2a3c),
			ANSI: ansi(0x2b1f28, 0xff6f8e, 0x9fd8a8, 0xf3c98b, 0x9fb6ff, 0xff8fb8, 0x8fd8d0, 0xeedde6,
				0x6a5562, 0xff93ab, 0xbdeac3, 0xffdcaa, 0xbccbff, 0xffb3cf, 0xb0e8e1, 0xffffff),
			Match: hex(0x2a3a6e), MatchCur: hex(0xf3c98b), MatchMark: hex(0x9a86e0),
		},
		UI: Chrome{
			Chrome: hex(0x140e13), Active: hex(0x1a1218), ActiveFG: hex(0xffffff), Muted: hex(0xa8909e),
			Border: hex(0x2b1f28), Accent: hex(0xff8fb8), StatusFG: hex(0x9c8492), Folder: hex(0xeedde6),
			Branch: hex(0x9fd8a8), CapGlyph: hex(0xa8909e), CapHover: hex(0x372a34), CapPress: hex(0x211720),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0xf3c98b), Done: hex(0x9fd8a8),
			ToastBG: hex(0x24181f), ToastLine: hex(0x2b1f28), ToastFG: hex(0xeedde6), ToastGo: hex(0xff8fb8),
			ToastTitle: hex(0xffffff), UsageHigh: hex(0xff93ab),
			DiffAddBG: hex(0x1c2a20), DiffAddFG: hex(0xbdeac3), DiffDelBG: hex(0x2e1820), DiffDelFG: hex(0xffb3c6),
			CodeKeyword: hex(0xff8fb8), CodeString: hex(0xa8e0b0), CodeComment: hex(0xa8909e),
			CodeNumber: hex(0xf3c98b), CodeName: hex(0x9fb6ff),
		},
	}
}

// Matcha: olive-black with a soft green accent.
func matcha() Theme {
	return Theme{
		Key: "matcha", Name: "Matcha",
		Term: render.Palette{
			FG: hex(0xdde3cf), BG: hex(0x12160f), Cursor: hex(0x9cc77a), Selection: hex(0x2c3a22),
			ANSI: ansi(0x1d2219, 0xe0786a, 0x9cc77a, 0xd9c27a, 0x86a8c9, 0xc497b0, 0x88c4ae, 0xdde3cf,
				0x5a6350, 0xf09485, 0xb6dd96, 0xebd796, 0xa4c2de, 0xd8b2c7, 0xa6dcc8, 0xf4f7ec),
			Match: hex(0x2f3a5e), MatchCur: hex(0xd9c27a), MatchMark: hex(0x8a9ad0),
		},
		UI: Chrome{
			Chrome: hex(0x0e110b), Active: hex(0x12160f), ActiveFG: hex(0xffffff), Muted: hex(0x8e9783),
			Border: hex(0x232a1c), Accent: hex(0x9cc77a), StatusFG: hex(0x838c78), Folder: hex(0xdde3cf),
			Branch: hex(0xb6dd96), CapGlyph: hex(0x8e9783), CapHover: hex(0x283020), CapPress: hex(0x181d13),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0xd9c27a), Done: hex(0x9cc77a),
			ToastBG: hex(0x181d14), ToastLine: hex(0x283020), ToastFG: hex(0xdde3cf), ToastGo: hex(0x9cc77a),
			ToastTitle: hex(0xffffff), UsageHigh: hex(0xf09485),
			DiffAddBG: hex(0x1a2a17), DiffAddFG: hex(0xb6dd96), DiffDelBG: hex(0x2c1715), DiffDelFG: hex(0xf0a093),
			CodeKeyword: hex(0xb6dd96), CodeString: hex(0xd9c27a), CodeComment: hex(0x8e9783),
			CodeNumber: hex(0xf09485), CodeName: hex(0x86a8c9),
		},
	}
}

// Kōyō (autumn leaves): brown-black with a maple orange accent.
func koyo() Theme {
	return Theme{
		Key: "koyo", Name: "Kōyō",
		Term: render.Palette{
			FG: hex(0xeadccd), BG: hex(0x17110d), Cursor: hex(0xe8803a), Selection: hex(0x45291a),
			ANSI: ansi(0x241b15, 0xe0604a, 0xa8b86a, 0xe8b84a, 0x8aa0b8, 0xc98a9a, 0x8ab8a8, 0xeadccd,
				0x6a5848, 0xf47e66, 0xc2d086, 0xf4cc76, 0xa8bcd2, 0xdea6b4, 0xa6d0c2, 0xfaf1e6),
			Match: hex(0x2e3658), MatchCur: hex(0xe8b84a), MatchMark: hex(0x8a94c8),
		},
		UI: Chrome{
			Chrome: hex(0x110c09), Active: hex(0x17110d), ActiveFG: hex(0xffffff), Muted: hex(0xa08c7a),
			Border: hex(0x2a1f17), Accent: hex(0xe8803a), StatusFG: hex(0x958170), Folder: hex(0xeadccd),
			Branch: hex(0xa8b86a), CapGlyph: hex(0xa08c7a), CapHover: hex(0x33261c), CapPress: hex(0x1e1611),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0xf0c060), Done: hex(0xa8b86a),
			ToastBG: hex(0x1f1712), ToastLine: hex(0x2e2219), ToastFG: hex(0xeadccd), ToastGo: hex(0xe8803a),
			ToastTitle: hex(0xffffff), UsageHigh: hex(0xf47e66),
			DiffAddBG: hex(0x1f2716), DiffAddFG: hex(0xc2d086), DiffDelBG: hex(0x30170f), DiffDelFG: hex(0xf4a08a),
			CodeKeyword: hex(0xe8803a), CodeString: hex(0xc2d086), CodeComment: hex(0xa08c7a),
			CodeNumber: hex(0xf4cc76), CodeName: hex(0x9ab4cc),
		},
	}
}

// Hai (ash): a calm mid-dark grey with muted colors, for low glare.
func hai() Theme {
	return Theme{
		Key: "hai", Name: "Hai",
		Term: render.Palette{
			FG: hex(0xd0d2d5), BG: hex(0x26282b), Cursor: hex(0xa6bcd8), Selection: hex(0x3c4250),
			ANSI: ansi(0x33363a, 0xd88888, 0x98b890, 0xc8b07a, 0x8aa4c4, 0xb49ac0, 0x88b4b4, 0xd0d2d5,
				0x70747a, 0xe09a9a, 0xb0cca8, 0xdcc698, 0xa6bcd8, 0xc8b2d4, 0xa2cccc, 0xe6e7e9),
			Match: hex(0x644a82), MatchCur: hex(0xdcc698), MatchMark: hex(0x9a8ad0),
		},
		UI: Chrome{
			Chrome: hex(0x1f2124), Active: hex(0x26282b), ActiveFG: hex(0xf2f3f4), Muted: hex(0x969aa0),
			Border: hex(0x34373b), Accent: hex(0x8aa4c4), StatusFG: hex(0x8c9096), Folder: hex(0xd0d2d5),
			Branch: hex(0x98b890), CapGlyph: hex(0x969aa0), CapHover: hex(0x3a3d42), CapPress: hex(0x2c2f33),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0xdcc698), Done: hex(0x98b890),
			ToastBG: hex(0x2d3034), ToastLine: hex(0x3c4046), ToastFG: hex(0xd6d8db), ToastGo: hex(0xa6bcd8),
			ToastTitle: hex(0xf2f3f4), UsageHigh: hex(0xe09a9a),
			DiffAddBG: hex(0x2c3a2e), DiffAddFG: hex(0xb8d8b0), DiffDelBG: hex(0x40292b), DiffDelFG: hex(0xf0b0b0),
			CodeKeyword: hex(0xb49ac0), CodeString: hex(0xb0cca8), CodeComment: hex(0xa0a4aa),
			CodeNumber: hex(0xdcc698), CodeName: hex(0x9ab4d4),
		},
	}
}

// Fuji (wisteria): deep violet-black with a lilac accent.
func fuji() Theme {
	return Theme{
		Key: "fuji", Name: "Fuji",
		Term: render.Palette{
			FG: hex(0xe2dcec), BG: hex(0x15131f), Cursor: hex(0xc4a7e7), Selection: hex(0x4a2e5c),
			ANSI: ansi(0x221f2e, 0xf07c8c, 0x9cd4a4, 0xecc98a, 0x8fa8f0, 0xc4a7e7, 0x8cd0d4, 0xe2dcec,
				0x5e5874, 0xff98a6, 0xb8e6be, 0xf6dca8, 0xadc0ff, 0xd8c2f2, 0xaae2e4, 0xf8f5fc),
			Match: hex(0x16506a), MatchCur: hex(0xecc98a), MatchMark: hex(0x7fa8d8),
		},
		UI: Chrome{
			Chrome: hex(0x100f19), Active: hex(0x15131f), ActiveFG: hex(0xffffff), Muted: hex(0x9a92ae),
			Border: hex(0x262236), Accent: hex(0xc4a7e7), StatusFG: hex(0x8f88a4), Folder: hex(0xe2dcec),
			Branch: hex(0x9cd4a4), CapGlyph: hex(0x9a92ae), CapHover: hex(0x2e2940), CapPress: hex(0x1d1a29),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0xecc98a), Done: hex(0x9cd4a4),
			ToastBG: hex(0x1d1a2a), ToastLine: hex(0x2c2740), ToastFG: hex(0xe2dcec), ToastGo: hex(0xc4a7e7),
			ToastTitle: hex(0xffffff), UsageHigh: hex(0xff98a6),
			DiffAddBG: hex(0x18291f), DiffAddFG: hex(0xb8e6be), DiffDelBG: hex(0x2e1722), DiffDelFG: hex(0xffb0bc),
			CodeKeyword: hex(0xc4a7e7), CodeString: hex(0xb8e6be), CodeComment: hex(0x9a92ae),
			CodeNumber: hex(0xf6dca8), CodeName: hex(0x9fb4f8),
		},
	}
}

// Kiri (fog) is the second light theme: a cool grey-blue. Like Washi, its caption
// hover/press go darker, not lighter.
func kiri() Theme {
	return Theme{
		Key: "kiri", Name: "Kiri",
		Term: render.Palette{
			FG: hex(0x24303a), BG: hex(0xe8edf1), Cursor: hex(0x255d88), Selection: hex(0xc5d6e6),
			ANSI: ansi(0x24303a, 0xb83a3a, 0x2e7a4e, 0x8a6500, 0x2a62a8, 0x8a4aa8, 0x1a7880, 0xa9b4be,
				0x5f6b76, 0xcc4b4b, 0x37905c, 0xa87a00, 0x3a76c0, 0x9c5cbc, 0x228c94, 0x151b21),
			Match: hex(0xf2e2a8), MatchCur: hex(0x8a6500), MatchMark: hex(0x6a5fb8),
		},
		UI: Chrome{
			Chrome: hex(0xdde3e9), Active: hex(0xe8edf1), ActiveFG: hex(0x24303a), Muted: hex(0x56626d),
			Border: hex(0xcbd3db), Accent: hex(0x255d88), StatusFG: hex(0x5f6b76), Folder: hex(0x24303a),
			Branch: hex(0x2e7a4e), CapGlyph: hex(0x5f6b76), CapHover: hex(0xc6cfd8), CapPress: hex(0xbac4ce),
			CloseHover: closeHover, ClosePress: closePress, Needs: hex(0x8a6500), Done: hex(0x2e7a4e),
			ToastBG: hex(0xf7f9fb), ToastLine: hex(0xcbd3db), ToastFG: hex(0x24303a), ToastGo: hex(0x255d88),
			ToastTitle: hex(0x24303a), UsageHigh: hex(0xb83a3a),
			DiffAddBG: hex(0xcfe0d4), DiffAddFG: hex(0x1d5a37), DiffDelBG: hex(0xefd6d6), DiffDelFG: hex(0x962d2d),
			CodeKeyword: hex(0x6a2f8a), CodeString: hex(0x1d5a37), CodeComment: hex(0x4f5b66),
			CodeNumber: hex(0x7a5500), CodeName: hex(0x1f4f8f),
		},
	}
}
