package packets

import (
	"fmt"
	"io"
)

type UnsubackPacket struct {
	FixedHeader
	PacketID uint16
}

func (ua *UnsubackPacket) String() string {
	str := fmt.Sprintf("%s\n", ua.FixedHeader)
	str += fmt.Sprintf("PacketID: %d", ua.PacketID)
	return str
}

func (ua *UnsubackPacket) WriteTo(w io.Writer) error {
	var err error
	ua.FixedHeader.RemainingLength = 2
	packet := ua.FixedHeader.pack()
	packet.Write(encodeUint16(ua.PacketID))
	_, err = packet.WriteTo(w)

	return err
}

func (ua *UnsubackPacket) ReadFrom(r io.Reader) error {
	var err error
	ua.PacketID, err = decodeUint16(r)
	return err
}

func (ua *UnsubackPacket) Details() Details {
	return Details{Qos: 0, PacketID: ua.PacketID}
}
