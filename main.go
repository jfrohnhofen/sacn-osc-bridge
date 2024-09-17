package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
)

type SacnRootLayer struct {
	PreambleSize        uint16
	PostambleSize       uint16
	AcnPacketIdentifier [12]uint8
	FlagsAndLength      uint16
	Vector              uint32
	Cid                 [16]uint8
}

type SacnFramingLayer struct {
	FlagsAndLength uint16
	Vector         uint32
	Source         [64]uint8
	Priority       uint8
	SyncAddress    uint16
	SeqNumber      uint8
	Options        uint8
	Universe       uint16
}

type SacnDmpLayer struct {
	FlagsAndLength       uint16
	Vector               uint8
	AddressTypeDataType  uint8
	FirstPropertyAddress uint16
	AddressIncrement     uint16
	PropertyValueCount   uint16
	PropertyValues       [513]uint8
}

type SacnDataPacket struct {
	RootLayer    SacnRootLayer
	FramingLayer SacnFramingLayer
	DmpLayer     SacnDmpLayer
}

func main() {
	var (
		sacnIface  = flag.String("sacn-iface", "", "network interface to listen for sACN messages")
		universe   = flag.Uint("universe", 1, "sACN universe")
		dmxChannel = flag.Uint("dmx", 1, "DMX channel")
		oscAddress = flag.String("osc-address", "127.0.0.1:53000", "OSC address to send commands to")
		oscCommand = flag.String("osc-command", "/cue/%d/go", "OSC command template - %d is replaced by the received DMX value")
	)
	flag.Parse()

	var iface *net.Interface
	if *sacnIface != "" {
		var err error
		iface, err = net.InterfaceByName(*sacnIface)
		if err != nil {
			log.Fatal(err)
		}
	}
	addr := &net.UDPAddr{
		IP:   net.IP{239, 255, byte(*universe >> 8), byte(*universe)},
		Port: 5568,
	}

	conn, err := net.ListenMulticastUDP("udp", iface, addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	log.Printf("listening to %s:%s:%d\n", iface.Name, addr.IP, addr.Port)

	prevValue := uint8(0)
	for {
		buffer := make([]byte, 1024)
		_, err := conn.Read(buffer)
		if err != nil {
			log.Print(err)
			continue
		}

		var packet SacnDataPacket
		binary.Read(bytes.NewReader(buffer), binary.BigEndian, &packet)
		source, value, err := processPacket(packet, uint16(*universe), uint16(*dmxChannel))
		if err != nil {
			log.Printf("ignoring packet: %s\n", err)
			continue
		}

		log.Printf("received packet: source=%s dmx[%d]=%d\n", source, *dmxChannel, value)

		if value != prevValue {
			prevValue = value
			cmd := fmt.Sprintf(*oscCommand, value)
			if err := sendOscCommand(*oscAddress, cmd+"\x00,"); err != nil {
				log.Printf("failed to send command: %s\n", err)
				continue
			}
			log.Printf("sent command: dest=%s cmd=%s\n", *oscAddress, cmd)
		}
	}
}

func processPacket(packet SacnDataPacket, universe, dmxChannel uint16) (string, uint8, error) {
	// Root Layer checks
	if packet.RootLayer.PreambleSize != 0x0010 {
		return "", 0, errors.New("invalid sACN pre-amble size")
	}
	if packet.RootLayer.PostambleSize != 0x0000 {
		return "", 0, errors.New("invalid sACN post-amble size")
	}
	if packet.RootLayer.AcnPacketIdentifier != [12]byte{0x41, 0x53, 0x43, 0x2d, 0x45, 0x31, 0x2e, 0x31, 0x37, 0x00, 0x00, 0x00} {
		return "", 0, errors.New("invalid sACN packet identifier")
	}
	if flags := packet.RootLayer.FlagsAndLength & 0xf000; flags != 0x7000 {
		return "", 0, errors.New("invalid sACN root layer flags")
	}
	if packet.RootLayer.Vector != 0x00000004 {
		return "", 0, errors.New("invalid sACN root layer vector")
	}

	// Framing Layer checks
	if flags := packet.FramingLayer.FlagsAndLength & 0xf000; flags != 0x7000 {
		return "", 0, errors.New("invalid sACN framing layer flags")
	}
	if packet.FramingLayer.Vector != 0x00000002 {
		return "", 0, errors.New("invalid sACN framing layer vector")
	}
	if packet.FramingLayer.Options != 0x00 {
		return "", 0, errors.New("invalid sACN options")
	}
	if packet.FramingLayer.Universe != universe {
		return "", 0, errors.New("different sACN universe")
	}

	// DMP Layer checks
	if flags := packet.DmpLayer.FlagsAndLength & 0xf000; flags != 0x7000 {
		return "", 0, errors.New("invalid sACN DMP layer flags")
	}
	if packet.DmpLayer.Vector != 0x02 {
		return "", 0, errors.New("invalid sACN DMP layer vector")
	}
	if packet.DmpLayer.AddressTypeDataType != 0xa1 {
		return "", 0, errors.New("invalid sACN address type & data type")
	}
	if packet.DmpLayer.FirstPropertyAddress != 0x0000 {
		return "", 0, errors.New("invalid sACN first property address")
	}
	if packet.DmpLayer.AddressIncrement != 0x0001 {
		return "", 0, errors.New("invalid sACN address increment")
	}
	if packet.DmpLayer.PropertyValueCount <= dmxChannel {
		return "", 0, errors.New("selected DMX channel not part of sACN packet")
	}
	if packet.DmpLayer.PropertyValues[0] != 0x00 {
		return "", 0, errors.New("invalid sACN DMX start code")
	}

	return string(packet.FramingLayer.Source[:]), packet.DmpLayer.PropertyValues[dmxChannel], nil
}

func sendOscCommand(address string, cmd string) error {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(cmd))
	return err
}
