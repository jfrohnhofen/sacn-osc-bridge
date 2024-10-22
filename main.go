package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/jfrohnhofen/sacn-osc-bridge/sacn"
)

func main() {
	var (
		sacnIface    = flag.String("sacn-iface", "", "network interface to listen for sACN messages")
		sacnUniverse = flag.Uint("sacn-universe", 1, "sACN universe")
		dmxChannel   = flag.Uint("dmx-channel", 1, "DMX channel")
		oscAddress   = flag.String("osc-address", "127.0.0.1:53000", "OSC address to send commands to")
		oscCommand   = flag.String("osc-command", "/cue/%d/go", "OSC command template - %d is replaced by the received DMX value")
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
		IP:   net.IP{239, 255, byte(*sacnUniverse >> 8), byte(*sacnUniverse)},
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

		packet, err := sacn.ParsePacket(bytes.NewReader(buffer))
		if err != nil {
			log.Printf("invalid sACN packet: %s\n", err)
			continue
		}
		if uint(packet.FramingLayer.Universe) != *sacnUniverse {
			log.Printf("invalid sACN packet: unexpected universe\n")
			continue
		}
		if uint(packet.DmpLayer.PropertyValueCount) <= *dmxChannel {
			log.Printf("invalid sACN packet: not enough DMX values\n")
			continue
		}

		value := packet.DmpLayer.PropertyValues[*dmxChannel]
		source := string(packet.FramingLayer.Source[:])
		log.Printf("sACN packet: source=%s dmx[%d]=%d\n", source, *dmxChannel, value)

		if value == prevValue+1 {
			addr, err := net.ResolveUDPAddr("udp", *oscAddress)
			if err != nil {
				log.Printf("failed to resolve OSC address: %s", err)
				continue
			}
			conn, err := net.DialUDP("udp", nil, addr)
			if err != nil {
				log.Printf("failed to connect to OSC address: %s", err)
				continue
			}

			cmd := fmt.Sprintf(*oscCommand, value)
			_, err = conn.Write([]byte(cmd))
			if err != nil {
				log.Printf("failed to write OSC command: %s", err)
				continue
			}
			conn.Close()

			log.Printf("sent OSC command: dest=%s cmd=%s\n", *oscAddress, cmd)
		}

		prevValue = value
	}
}
