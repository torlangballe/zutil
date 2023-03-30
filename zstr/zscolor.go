package zstr

import "strings"

const (
	EscBlack   = "\x1B[30m"
	EscRed     = "\x1B[31m"
	EscGreen   = "\x1B[32m"
	EscYellow  = "\x1B[33m"
	EscBlue    = "\x1B[34m"
	EscMagenta = "\x1B[35m"
	EscCyan    = "\x1B[36m"
	EscWhite   = "\x1B[37m"
	EscNoColor = "\x1b[0m"
)

var ColorRemover = strings.NewReplacer(
	EscBlack, "",
	EscRed, "",
	EscGreen, "",
	EscYellow, "",
	EscBlue, "",
	EscMagenta, "",
	EscCyan, "",
	EscWhite, "",
	EscNoColor, "",
)

var ColorSetter = strings.NewReplacer(
	"🟥", EscRed,
	"🟩", EscGreen,
	"🟨", EscYellow,
	"🟦", EscBlue,
	"🟪", EscMagenta,
)

// termColor converts a 24-bit RGB color into a term256 compatible approximation.
func termColor(r, g, b uint16) uint16 {
	rterm := (((r * 5) + 127) / 255) * 36
	gterm := (((g * 5) + 127) / 255) * 6
	bterm := (((b * 5) + 127) / 255)

	return rterm + gterm + bterm + 16 + 1 // termbox default color offset
}

func GetColorEscapeCode(r, g, b int) string {
	R := r&128 > 0
	G := g&128 > 0
	B := b&128 > 0
	if R && G && B {
		return EscWhite
	}
	if !R && !G && !B {
		return EscBlack
	}
	if R && !G && !B {
		return EscRed
	}
	if !R && G && !B {
		return EscGreen
	}
	if !R && !G && B {
		return EscBlue
	}
	if R && G && !B {
		return EscYellow
	}
	if R && !G && B {
		return EscMagenta
	}
	if !R && G && B {
		return EscCyan
	}
	return ""
}
