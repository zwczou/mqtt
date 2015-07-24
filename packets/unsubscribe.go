package packets

import (
	"bytes"
	"fmt"
	"io"
)

type UnsubscribePacket struct {
	FixedHeader
	PacketID uint16
	Topics   []string
}

func (u *UnsubscribePacket) String() string {
	str := fmt.Sprintf("%s\n", u.FixedHeader)
	str += fmt.Sprintf("PacketID: %d", u.PacketID)
	return str
}

func (u *UnsubscribePacket) WriteTo(w io.Writer) error {
	var body bytes.Buffer
	var err error

	body.Write(encodeUint16(u.PacketID))
	for _, topic := range u.Topics {
		body.Write(encodeString(topic))
	}
	u.FixedHeader.RemainingLength = body.Len()
	packet := u.FixedHeader.pack()
	packet.Write(body.Bytes())
	_, err = packet.WriteTo(w)

	return err
}

func (u *UnsubscribePacket) ReadFrom(r io.Reader) error {
	var err error
	var topic string

	u.PacketID, err = decodeUint16(r)
	if err != nil {
		return nil
	}
	for topic, err = decodeString(r); err == nil; topic, err = decodeString(r) {
		u.Topics = append(u.Topics, topic)
	}
	return err
}

func (u *UnsubscribePacket) Details() Details {
	return Details{Qos: 1, PacketID: u.PacketID}
}
