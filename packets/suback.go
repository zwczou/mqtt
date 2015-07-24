package packets

import (
	"bytes"
	"fmt"
	"io"
)

type SubackPacket struct {
	FixedHeader
	PacketID    uint16
	GrantedQoss []byte
}

func (sa *SubackPacket) String() string {
	str := fmt.Sprintf("%s\n", sa.FixedHeader)
	str += fmt.Sprintf("PacketID: %d", sa.PacketID)
	return str
}

func (sa *SubackPacket) WriteTo(w io.Writer) error {
	var body bytes.Buffer
	var err error

	body.Write(encodeUint16(sa.PacketID))
	body.Write(sa.GrantedQoss)
	sa.FixedHeader.RemainingLength = body.Len()
	packet := sa.FixedHeader.pack()
	packet.Write(body.Bytes())
	_, err = packet.WriteTo(w)

	return err
}

func (sa *SubackPacket) ReadFrom(r io.Reader) error {
	var qosBuffer bytes.Buffer
	var err error

	sa.PacketID, err = decodeUint16(r)
	if err != nil {
		return err
	}
	_, err = qosBuffer.ReadFrom(r)
	sa.GrantedQoss = qosBuffer.Bytes()
	return err
}

func (sa *SubackPacket) Details() Details {
	return Details{Qos: 0, PacketID: sa.PacketID}
}
