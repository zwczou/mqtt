package packets

import (
	"bytes"
	"fmt"
	"io"
)

type SubscribePacket struct {
	FixedHeader
	PacketID uint16
	Topics   []string
	Qoss     []byte
}

func (s *SubscribePacket) String() string {
	str := fmt.Sprintf("%s\n", s.FixedHeader)
	str += fmt.Sprintf("PacketID: %d topics: %s", s.PacketID, s.Topics)
	return str
}

func (s *SubscribePacket) WriteTo(w io.Writer) error {
	var body bytes.Buffer
	var err error

	body.Write(encodeUint16(s.PacketID))
	for i, topic := range s.Topics {
		body.Write(encodeString(topic))
		body.WriteByte(s.Qoss[i])
	}
	s.FixedHeader.RemainingLength = body.Len()
	packet := s.FixedHeader.pack()
	packet.Write(body.Bytes())
	_, err = packet.WriteTo(w)

	return err
}

func (s *SubscribePacket) ReadFrom(r io.Reader) error {
	var err error

	s.PacketID, err = decodeUint16(r)
	if err != nil {
		return err
	}
	payloadLength := s.FixedHeader.RemainingLength - 2
	for payloadLength > 0 {
		topic, err := decodeString(r)
		if err != nil {
			return err
		}
		s.Topics = append(s.Topics, topic)
		qos, err := decodeByte(r)
		if err != nil {
			return err
		}
		s.Qoss = append(s.Qoss, qos)
		payloadLength -= 2 + len(topic) + 1 //2 bytes of string length, plus string, plus 1 byte for Qos
	}
	return nil
}

func (s *SubscribePacket) Details() Details {
	return Details{Qos: 1, PacketID: s.PacketID}
}
