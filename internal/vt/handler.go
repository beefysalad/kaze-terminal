// Package vt parses the byte stream a shell writes into terminal events.
package vt

// Handler receives parsed terminal events. Slices passed to CSI, ESC and OSC
// are reused by the parser; implementations must copy anything they keep.
type Handler interface {
	Print(r rune)
	// Execute is called for C0 controls: BEL, BS, HT, LF, VT, FF, CR and friends.
	Execute(b byte)
	// CSI is called for a complete control sequence. private is '?', '>', '<',
	// '=' or 0. Missing parameters are -1. Both ';' and ':' separate params.
	CSI(params []int, intermediates []byte, private byte, final byte)
	ESC(intermediates []byte, final byte)
	// OSC data excludes the leading "ESC ]" and the terminator.
	OSC(data []byte)
}

// SubParamHandler is optionally implemented by a Handler. When it is,
// the parser calls CSISub instead of CSI.
// Bit i of colon is set when params[i] was preceded by ':' rather than ';'.
type SubParamHandler interface {
	CSISub(params []int, colon uint32, intermediates []byte, private byte, final byte)
}
