package packets

import (
	"fmt"
	"io"
)

type PubcompPacket struct {
	FixedHeader
	PacketID uint16
}

func (pc *PubcompPacket) String() string {
	str := fmt.Sprintf("%s\n", pc.FixedHeader)
	str += fmt.Sprintf("PacketID: %d", pc.PacketID)
	return str
}

func (pc *PubcompPacket) WriteTo(w io.Writer) error {
	var err error
	pc.FixedHeader.RemainingLength = 2
	packet := pc.FixedHeader.pack()
	packet.Write(encodeUint16(pc.PacketID))
	_, err = packet.WriteTo(w)

	return err
}

func (pc *PubcompPacket) ReadFrom(r io.Reader) error {
	var err error
	pc.PacketID, err = decodeUint16(r)
	return err
}

func (pc *PubcompPacket) Details() Details {
	return Details{Qos: pc.Qos, PacketID: pc.PacketID}
}
