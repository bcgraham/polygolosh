package gba

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/howeyc/crc16"
)

const headerSize = 0xc0
const headerSize16 = headerSize / 2
const minSize = 0x100 + headerSize
const maxSize = 0x3FF40 + headerSize

type gamePak struct {
	hs   [headerSize16]uint16
	data []uint32
	crc  crc16.Hash16
}

func newGamePak(data []byte) (*gamePak, error) {
	data = addPadding(data)

	if len(data) > maxSize {
		return nil, fmt.Errorf("game pak data (%dkb) exceeds the maximum of 256kb", len(data)>>10)
	}
	if len(data) < minSize {
		return nil, errors.New("game pak data smaller than the minimum of 100 bytes")
	}

	r1 := bytes.NewReader(data)
	gp := &gamePak{crc: NewCRC()}
	err := binary.Read(r1, binary.LittleEndian, gp.hs[:])
	if err != nil {
		return gp, err
	}

	r2 := io.TeeReader(r1, gp.crc)
	gp.data = make([]uint32, r1.Len()/4)
	err = binary.Read(r2, binary.LittleEndian, gp.data)
	if err != nil {
		return gp, err
	}
	return gp, nil
}

func (gP *gamePak) Headers() [headerSize16]uint16 {
	return gP.hs
}

func (gP *gamePak) Data() []uint32 {
	return gP.data
}

func (gP *gamePak) CRC() crc16.Hash16 {
	return gP.crc
}

func (gP *gamePak) Len() int {
	return len(gP.data)
}

func addPadding(bs []byte) []byte {
	l := len(bs)
	pl := paddedLength(l)
	padding := make([]byte, pl-l)
	return append(bs, padding...)
}

// paddedLength rounds up to the nearest multiple of 16.
func paddedLength(x int) int {
	return (x + 0xf) & -0x10
}
