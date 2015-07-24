package packets

import (
	"fmt"
	"io"
)

type PubrelPacket struct {
	FixedHeader
	PacketID uint16
}

func (pr *PubrelPacket) String() string {
	str := fmt.Sprintf("%s\n", pr.FixedHeader)
	str += fmt.Sprintf("PacketID: %d", pr.PacketID)
	return str
}

func (pr *PubrelPacket) WriteTo(w io.Writer) error {
	var err error
	pr.FixedHeader.RemainingLength = 2
	packet := pr.FixedHeader.pack()
	packet.Write(encodeUint16(pr.PacketID))
	_, err = packet.WriteTo(w)

	return err
}

func (pr *PubrelPacket) ReadFrom(r io.Reader) error {
	var err error
	pr.PacketID, err = decodeUint16(r)
	return err
}

func (pr *PubrelPacket) Details() Details {
	return Details{Qos: pr.Qos, PacketID: pr.PacketID}
}
