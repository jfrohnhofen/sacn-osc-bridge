package sacn

import (
	"encoding/binary"
	"errors"
	"io"
)

type RootLayer struct {
	PreambleSize        uint16
	PostambleSize       uint16
	AcnPacketIdentifier [12]uint8
	FlagsAndLength      uint16
	Vector              uint32
	Cid                 [16]uint8
}

type FramingLayer struct {
	FlagsAndLength uint16
	Vector         uint32
	Source         [64]uint8
	Priority       uint8
	SyncAddress    uint16
	SeqNumber      uint8
	Options        uint8
	Universe       uint16
}

type DmpLayer struct {
	FlagsAndLength       uint16
	Vector               uint8
	AddressTypeDataType  uint8
	FirstPropertyAddress uint16
	AddressIncrement     uint16
	PropertyValueCount   uint16
	PropertyValues       [513]uint8
}

type DataPacket struct {
	RootLayer    RootLayer
	FramingLayer FramingLayer
	DmpLayer     DmpLayer
}

func ParsePacket(r io.Reader) (*DataPacket, error) {
	var packet DataPacket
	if err := binary.Read(r, binary.BigEndian, &packet); err != nil {
		return nil, err
	}

	// Root Layer checks
	if packet.RootLayer.PreambleSize != 0x0010 {
		return nil, errors.New("invalid sACN pre-amble size")
	}
	if packet.RootLayer.PostambleSize != 0x0000 {
		return nil, errors.New("invalid sACN post-amble size")
	}
	if packet.RootLayer.AcnPacketIdentifier != [12]byte{0x41, 0x53, 0x43, 0x2d, 0x45, 0x31, 0x2e, 0x31, 0x37, 0x00, 0x00, 0x00} {
		return nil, errors.New("invalid sACN packet identifier")
	}
	if flags := packet.RootLayer.FlagsAndLength & 0xf000; flags != 0x7000 {
		return nil, errors.New("invalid sACN root layer flags")
	}
	if packet.RootLayer.Vector != 0x00000004 {
		return nil, errors.New("invalid sACN root layer vector")
	}

	// Framing Layer checks
	if flags := packet.FramingLayer.FlagsAndLength & 0xf000; flags != 0x7000 {
		return nil, errors.New("invalid sACN framing layer flags")
	}
	if packet.FramingLayer.Vector != 0x00000002 {
		return nil, errors.New("invalid sACN framing layer vector")
	}
	if packet.FramingLayer.Options != 0x00 {
		return nil, errors.New("invalid sACN options")
	}

	// DMP Layer checks
	if flags := packet.DmpLayer.FlagsAndLength & 0xf000; flags != 0x7000 {
		return nil, errors.New("invalid sACN DMP layer flags")
	}
	if packet.DmpLayer.Vector != 0x02 {
		return nil, errors.New("invalid sACN DMP layer vector")
	}
	if packet.DmpLayer.AddressTypeDataType != 0xa1 {
		return nil, errors.New("invalid sACN address type & data type")
	}
	if packet.DmpLayer.FirstPropertyAddress != 0x0000 {
		return nil, errors.New("invalid sACN first property address")
	}
	if packet.DmpLayer.AddressIncrement != 0x0001 {
		return nil, errors.New("invalid sACN address increment")
	}
	if packet.DmpLayer.PropertyValues[0] != 0x00 {
		return nil, errors.New("invalid sACN DMX start code")
	}

	return &packet, nil
}
