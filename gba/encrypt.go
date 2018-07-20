package gba

const blockSize = 32
const stepSize = blockSize / 8

const lcg_a = 0x6f646573
const lcg_c = 1
const ctrStart = -0x20000c0
const key = 0x43202f2f

func NewGBACipher(iv uint32) *gbaCipher {
	pos := ctrStart
	return &gbaCipher{
		lcg: iv,
		pos: uint32(pos),
		key: key,
	}
}

type gbaCipher struct {
	lcg uint32
	pos uint32
	key uint32
}

func (g *gbaCipher) Encrypt(src uint32) uint32 {
	g.lcg = lcg_a*g.lcg + lcg_c
	dst := src ^ g.lcg ^ g.pos ^ g.key
	g.pos -= stepSize
	return dst
}
