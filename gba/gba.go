package gba

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/howeyc/crc16"
)

type GBA struct {
	Port      io.ReadWriteCloser
	handshake byte
	lcgSeed   byte
	finalCRC  byte
}

func (g *GBA) Multiboot(data []byte) error {
	gamepak, err := newGamePak(data)
	if err != nil {
		return err
	}

	if err := g.discover(); err != nil {
		return err
	}

	if err := g.stabilizeConnection(); err != nil {
		return err
	}

	if err := g.sendFirstPrimaryReplica(); err != nil {
		return err
	}

	headers := gamepak.Headers()
	if err := g.sendHeaders(headers); err != nil {
		return err
	}

	if err := g.sendSecondPrimaryReplica(); err != nil {
		return err
	}

	if err := g.sendPaletteData(); err != nil {
		return err
	}

	if err := g.sendHandshake(); err != nil {
		return err
	}

	time.Sleep(62500 * time.Microsecond)

	lengthInfo := gamepak.Len()
	if err := g.sendLengthInfo(lengthInfo); err != nil {
		return err
	}

	mainData := gamepak.Data()
	if err := g.sendMainData(mainData); err != nil {
		return err
	}

	if err := g.prepareForCRC(); err != nil {
		return err
	}

	crc := gamepak.CRC()
	return g.sendCRC(crc)
}

func (g *GBA) discover() error {
	var gbaInReplicaMode bool
Loop:
	for {
		time.Sleep(62500 * time.Microsecond)
		r, err := g.xfer16(0x6200)
		if err != nil {
			return err
		}
		switch r {
		case 0xFFFF:
			gbaInReplicaMode = true
		case 0x0:
			if gbaInReplicaMode {
				break Loop
			}
			gbaInReplicaMode = true
		case 0x7202:
			break Loop
		}
	}
	log.Printf("Discovered...\n")
	return nil
}

// TODO: The lower 8 bits of the response here
// should not be assumed to be 02, but saved.
// That means comparing it to the last response
// for i > 0.
func (g *GBA) stabilizeConnection() error {
	var i int
	for i < 15 {
		r, err := g.xfer16(0x6200)
		if err != nil {
			return err
		}
		if r == 0x7202 {
			i++
		} else {
			time.Sleep(62500 * time.Microsecond)
			i = 0
		}
	}
	log.Printf("Stabilized...\n")
	return nil
}

func (g *GBA) sendFirstPrimaryReplica() error {
	if _, err := g.sendDemanding(0x6102, 0x7202, 0xffff); err != nil {
		return err
	}
	log.Printf("Exchanged primary/replica info...\n")
	return nil
}

func (g *GBA) sendSecondPrimaryReplica() error {
	_, err := g.sendDemanding(0x6202, 0x7202, 0xffff)
	return err
}

func (g *GBA) sendHeaders(headers [headerSize16]uint16) error {
	clientBit := uint16(0x02)
	for i, hb := range headers {
		framesRemaining := headerSize16 - uint16(i)
		response := (framesRemaining << 8) | clientBit
		if _, err := g.sendDemanding(hb, response, 0xffff); err != nil {
			return err
		}
	}
	if _, err := g.sendDemanding(0x6200, clientBit, 0xffff); err != nil {
		return err
	}
	log.Printf("Headers sent...\n")
	return nil
}

func (g *GBA) sendPaletteData() error {
	r, err := g.sendUntilResponse(0x63d1, 0x7300, 0xff00, 0)
	if err != nil {
		return err
	}
	g.lcgSeed = byte(r & 0xff)
	log.Printf("Palette data sent...\n")
	return nil
}

func (g *GBA) sendHandshake() error {
	g.handshake = (g.lcgSeed + 0xff + 0xff + 0x11) & 0xff
	handshake := 0x6400 | uint16(g.handshake)
	if _, err := g.sendDemanding(handshake, 0x7300, 0xff00); err != nil {
		return err
	}
	log.Printf("Handshake sent...\n")
	return nil
}

func (g *GBA) sendLengthInfo(length int) error {
	lengthInfo := uint16(length) - 0x34
	r, err := g.sendDemanding(lengthInfo, 0x7300, 0xff00)
	if err != nil {
		return err
	}
	g.finalCRC = byte(r & 0xff)
	log.Printf("Length info sent...\n")
	return nil
}

func (g *GBA) sendMainData(data []uint32) error {
	log.Printf("Sending main data...\n")
	iv := []byte{0xd1, g.lcgSeed, 0xff, 0xff}
	gc := NewGBACipher(binary.LittleEndian.Uint32(iv))
	for i, p := range data {
		c := gc.Encrypt(p)
		if _, err := g.xfer32(c); err != nil {
			return err
		}
		fmt.Printf("%s%3.1f%%...", strings.Repeat("\r", 50), 100.0*float32(i)/float32(len(data)))
	}
	log.Printf("Main data sent...\n")
	return nil
}

func (g *GBA) prepareForCRC() error {
	delay := 62500 * time.Microsecond
	if _, err := g.sendUntilResponse(0x65, 0x75, 0xff, delay); err != nil {
		return err
	}
	_, err := g.sendDemanding(0x66, 0x75, 0xff)
	return err
}

func (g *GBA) sendCRC(crc crc16.Hash16) error {
	final := []byte{g.handshake, g.finalCRC, 0xff, 0xff}
	crc.Write(final)
	_, err := g.sendDemanding(crc.Sum16(), crc.Sum16(), 0xffff)
	return err
}

func (g *GBA) sendUntilResponse(w, c, mask uint16, delay time.Duration) (uint16, error) {
	for {
		r, err := g.xfer16(w)
		if err != nil {
			return r, err
		}
		if (r & mask) == (c & mask) {
			return r, nil
		}
		time.Sleep(delay)
	}
}

func (g *GBA) sendDemanding(w, c, mask uint16) (uint16, error) {
	r, err := g.xfer16(w)
	if err != nil {
		return r, err
	}
	if (r & mask) != (c & mask) {
		return r, errors.New("unexpected response")
	}
	return r, nil
}

func (g *GBA) xfer16(w uint16) (uint16, error) {
	return g.xfer32(uint32(w))
}

func (g *GBA) xfer32(w uint32) (uint16, error) {
	var r uint32
	err := g.xfer(w, &r)
	return uint16(r >> 16), err
}

func (g *GBA) xfer(w interface{}, r interface{}) error {
	if err := binary.Write(g.Port, binary.BigEndian, w); err != nil {
		return err
	}
	if err := binary.Read(g.Port, binary.BigEndian, r); err != nil {
		return err
	}
	return nil
}
