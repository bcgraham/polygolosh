package gba

import (
	"math/bits"

	"github.com/npat-efault/crc16"
)

const initial uint16 = 0xc387
const polynomial uint16 = 0xc37b

func NewCRC() crc16.Hash16 {
	poly := polynomial
	poly = bits.Reverse16(poly)
	conf := &crc16.Conf{
		Poly:   poly,
		BitRev: true,
		IniVal: initial,
	}
	return crc16.New(conf)
}
