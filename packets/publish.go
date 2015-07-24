package packets

import (
	"bytes"
	"fmt"
	"io"
)

type PublishPacket struct {
	FixedHeader
	TopicName string
	PacketID  uint16
	Payload   []byte
}

func (p *PublishPacket) String() string {
	str := fmt.Sprintf("%s\n", p.FixedHeader)
	str += fmt.Sprintf("topicName: %s PacketID: %d\n", p.TopicName, p.PacketID)
	str += fmt.Sprintf("payload: %s\n", string(p.Payload))
	return str
}

func (p *PublishPacket) WriteTo(w io.Writer) error {
	var body bytes.Buffer
	var err error

	body.Write(encodeString(p.TopicName))
	if p.Qos > 0 {
		body.Write(encodeUint16(p.PacketID))
	}
	p.FixedHeader.RemainingLength = body.Len() + len(p.Payload)
	packet := p.FixedHeader.pack()
	packet.Write(body.Bytes())
	packet.Write(p.Payload)
	_, err = w.Write(packet.Bytes())

	return err
}

func (p *PublishPacket) ReadFrom(r io.Reader) error {
	var err error
	var payloadLength = p.FixedHeader.RemainingLength

	p.TopicName, err = decodeString(r)
	if err != nil {
		return err
	}
	if p.Qos > 0 {
		p.PacketID, err = decodeUint16(r)
		if err != nil {
			return err
		}
		payloadLength -= len(p.TopicName) + 4
	} else {
		payloadLength -= len(p.TopicName) + 2
	}
	p.Payload = make([]byte, payloadLength)
	_, err = io.ReadFull(r, p.Payload)
	return err
}

func (p *PublishPacket) Copy() *PublishPacket {
	newP := NewControlPacket(Publish).(*PublishPacket)
	newP.TopicName = p.TopicName
	newP.Payload = p.Payload

	return newP
}

func (p *PublishPacket) Details() Details {
	return Details{Qos: p.Qos, PacketID: p.PacketID}
}
