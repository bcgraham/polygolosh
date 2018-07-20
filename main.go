package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"

	"github.com/bcgraham/gba-multiboot/gba"
	"github.com/jacobsa/go-serial/serial"
)

var port io.ReadWriteCloser
var debug bool
var file string
var portname string

func main() {
	flag.StringVar(&file, "file", "./mb.gba", "gba file")
	flag.StringVar(&portname, "portname", "/dev/tty.usbmodem1411", "port name")
	flag.Parse()
	options := serial.OpenOptions{
		PortName:              portname,
		BaudRate:              115200,
		DataBits:              8,
		StopBits:              1,
		MinimumReadSize:       2,
		InterCharacterTimeout: 100,
	}

	port, err := serial.Open(options)
	if err != nil {
		log.Fatalf("serial.Open: %v", err)
	}
	defer port.Close()
	g := &gba.GBA{Port: port}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Multiboot(data); err != nil {
		log.Fatal(err)
	}
}
