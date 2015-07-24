package packets

import (
	"fmt"
	"io"
)

type PubackPacket struct {
	FixedHeader
	PacketID uint16
}

func (pa *PubackPacket) String() string {
	str := fmt.Sprintf("%s\n", pa.FixedHeader)
	str += fmt.Sprintf("messageID: %d", pa.PacketID)
	return str
}

func (pa *PubackPacket) WriteTo(w io.Writer) error {
	var err error
	pa.FixedHeader.RemainingLength = 2
	packet := pa.FixedHeader.pack()
	packet.Write(encodeUint16(pa.PacketID))
	_, err = packet.WriteTo(w)

	return err
}

func (pa *PubackPacket) ReadFrom(r io.Reader) error {
	var err error
	pa.PacketID, err = decodeUint16(r)
	return err
}

func (pa *PubackPacket) Details() Details {
	return Details{Qos: pa.Qos, PacketID: pa.PacketID}
}
