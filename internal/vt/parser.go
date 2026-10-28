package vt

import "unicode/utf8"

const (
	maxParams = 32
	maxInter  = 4
	maxOSC    = 64 * 1024
	maxParam  = 99999999 // stops int overflow on absurd digit runs
)

type state uint8

const (
	stGround state = iota
	stEscape
	stEscInter
	stCSIEntry
	stCSIParam
	stCSIInter
	stCSIIgnore
	stOSC
	stOSCEsc
	stStrIgnore // DCS, SOS, PM, APC: consumed until ST
	stStrIgnoreEsc
)

// Parser is a Paul Williams style DEC parser with UTF-8 decoding in ground.
type Parser struct {
	h     Handler
	sub   SubParamHandler // non-nil when h wants the colon mask
	state state

	// UTF-8 assembly across Feed calls.
	u8     [4]byte
	u8len  int
	u8need int

	params    [maxParams]int
	nparams   int
	colon     uint32 // bit i: params[i] followed a ':'
	sepColon  bool   // the last separator was ':'
	cur       int
	hasDigits bool
	inter     [maxInter]byte
	ninter    int
	private   byte

	osc []byte
}

func NewParser(h Handler) *Parser {
	sub, _ := h.(SubParamHandler)
	return &Parser{h: h, sub: sub, osc: make([]byte, 0, 256)}
}

// Feed consumes bytes, calling the handler as sequences complete. State
// persists between calls, so input may be split anywhere.
func (p *Parser) Feed(data []byte) {
	for _, b := range data {
		// Hot path: printable ASCII in ground with no UTF-8 pending.
		if p.state == stGround && p.u8need == 0 && b >= 0x20 && b < 0x7f {
			p.h.Print(rune(b))
			continue
		}
		p.step(b)
	}
}

func (p *Parser) step(b byte) {
	switch p.state {
	case stGround:
		p.ground(b)
	case stEscape:
		p.escape(b)
	case stEscInter:
		p.escInter(b)
	case stCSIEntry, stCSIParam, stCSIInter, stCSIIgnore:
		p.csi(b)
	case stOSC:
		p.oscByte(b)
	case stOSCEsc:
		if b == '\\' {
			p.dispatchOSC()
			p.state = stGround
			return
		}
		// ESC not followed by ST: end the string and treat b as an escape byte.
		p.dispatchOSC()
		p.beginEscape()
		p.escape(b)
	case stStrIgnore:
		switch b {
		case 0x1b:
			p.state = stStrIgnoreEsc
		case 0x18, 0x1a:
			p.state = stGround
		}
	case stStrIgnoreEsc:
		if b == '\\' {
			p.state = stGround
			return
		}
		p.beginEscape()
		p.escape(b)
	}
}

func (p *Parser) ground(b byte) {
	if p.u8need > 0 {
		if b&0xc0 == 0x80 {
			p.u8[p.u8len] = b
			p.u8len++
			if p.u8len == p.u8need {
				r, _ := utf8.DecodeRune(p.u8[:p.u8len])
				p.u8need = 0
				p.h.Print(r) // RuneError for overlong or surrogate forms
			}
			return
		}
		// Truncated sequence: emit one replacement and reprocess b.
		p.u8need = 0
		p.h.Print(utf8.RuneError)
	}
	switch {
	case b == 0x1b:
		p.beginEscape()
	case b < 0x20:
		if b != 0x18 && b != 0x1a {
			p.h.Execute(b)
		}
	case b < 0x7f:
		p.h.Print(rune(b))
	case b == 0x7f:
		// DEL is ignored.
	case b >= 0xc2 && b <= 0xdf:
		p.startUTF8(b, 2)
	case b >= 0xe0 && b <= 0xef:
		p.startUTF8(b, 3)
	case b >= 0xf0 && b <= 0xf4:
		p.startUTF8(b, 4)
	default:
		p.h.Print(utf8.RuneError)
	}
}

func (p *Parser) startUTF8(b byte, need int) {
	p.u8[0] = b
	p.u8len = 1
	p.u8need = need
}

func (p *Parser) beginEscape() {
	p.state = stEscape
	p.ninter = 0
}

func (p *Parser) escape(b byte) {
	switch {
	case b == 0x1b:
		p.beginEscape()
	case b == 0x18 || b == 0x1a:
		p.state = stGround
	case b < 0x20:
		p.h.Execute(b)
	case b <= 0x2f:
		p.collectInter(b)
		p.state = stEscInter
	case b == '[':
		p.clearCSI()
		p.state = stCSIEntry
	case b == ']':
		p.osc = p.osc[:0]
		p.state = stOSC
	case b == 'P' || b == 'X' || b == '^' || b == '_':
		p.state = stStrIgnore
	case b <= 0x7e:
		p.h.ESC(p.inter[:p.ninter], b)
		p.state = stGround
	case b == 0x7f:
	default:
		p.state = stGround
		p.ground(b)
	}
}

func (p *Parser) escInter(b byte) {
	switch {
	case b == 0x1b:
		p.beginEscape()
	case b == 0x18 || b == 0x1a:
		p.state = stGround
	case b < 0x20:
		p.h.Execute(b)
	case b <= 0x2f:
		p.collectInter(b)
	case b <= 0x7e:
		p.h.ESC(p.inter[:p.ninter], b)
		p.state = stGround
	case b == 0x7f:
	default:
		p.state = stGround
		p.ground(b)
	}
}

func (p *Parser) collectInter(b byte) {
	if p.ninter < maxInter {
		p.inter[p.ninter] = b
		p.ninter++
	}
}

func (p *Parser) clearCSI() {
	p.nparams = 0
	p.colon = 0
	p.sepColon = false
	p.cur = 0
	p.hasDigits = false
	p.ninter = 0
	p.private = 0
}

func (p *Parser) pushParam() {
	if p.nparams < maxParams {
		if p.sepColon {
			p.colon |= 1 << uint(p.nparams)
		}
		if p.hasDigits {
			p.params[p.nparams] = p.cur
		} else {
			p.params[p.nparams] = -1
		}
		p.nparams++
	}
	p.cur = 0
	p.hasDigits = false
	p.sepColon = false
}

func (p *Parser) csi(b byte) {
	switch {
	case b == 0x1b:
		p.beginEscape()
		return
	case b == 0x18 || b == 0x1a:
		p.state = stGround
		return
	case b < 0x20:
		p.h.Execute(b)
		return
	case b == 0x7f:
		return
	case b >= 0x80:
		p.state = stGround
		p.ground(b)
		return
	}

	if b >= 0x40 {
		if p.state != stCSIIgnore {
			p.pushParam()
			if p.sub != nil {
				p.sub.CSISub(p.params[:p.nparams], p.colon, p.inter[:p.ninter], p.private, b)
			} else {
				p.h.CSI(p.params[:p.nparams], p.inter[:p.ninter], p.private, b)
			}
		}
		p.state = stGround
		return
	}

	switch p.state {
	case stCSIIgnore:
	case stCSIEntry, stCSIParam:
		switch {
		case b >= '0' && b <= '9':
			if p.cur < maxParam {
				p.cur = p.cur*10 + int(b-'0')
			}
			p.hasDigits = true
			p.state = stCSIParam
		case b == ';' || b == ':': // ':' is flattened into params and marked in colon
			p.pushParam()
			p.sepColon = b == ':'
			p.state = stCSIParam
		case b >= 0x3c: // '<' '=' '>' '?'
			if p.state == stCSIEntry {
				p.private = b
				p.state = stCSIParam
			} else {
				p.state = stCSIIgnore
			}
		default: // 0x20-0x2f
			p.collectInter(b)
			p.state = stCSIInter
		}
	case stCSIInter:
		if b <= 0x2f {
			p.collectInter(b)
		} else {
			p.state = stCSIIgnore
		}
	}
}

func (p *Parser) oscByte(b byte) {
	switch {
	case b == 0x07:
		p.dispatchOSC()
		p.state = stGround
	case b == 0x1b:
		p.state = stOSCEsc
	case b == 0x18 || b == 0x1a:
		p.state = stGround
	case b < 0x20:
	default:
		if len(p.osc) < maxOSC {
			p.osc = append(p.osc, b)
		}
	}
}

func (p *Parser) dispatchOSC() {
	p.h.OSC(p.osc)
	p.osc = p.osc[:0]
}
