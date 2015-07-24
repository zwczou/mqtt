package packets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Details struct {
	Qos      byte
	PacketID uint16
}

type ControlPacket interface {
	WriteTo(io.Writer) error
	ReadFrom(io.Reader) error
	String() string
	Details() Details
}

const (
	Connect     = 1
	Connack     = 2
	Publish     = 3
	Puback      = 4
	Pubrec      = 5
	Pubrel      = 6
	Pubcomp     = 7
	Subscribe   = 8
	Suback      = 9
	Unsubscribe = 10
	Unsuback    = 11
	Pingreq     = 12
	Pingresp    = 13
	Disconnect  = 14
)

var PacketNames = map[uint8]string{
	1:  "CONNECT",
	2:  "CONNACK",
	3:  "PUBLISH",
	4:  "PUBACK",
	5:  "PUBREC",
	6:  "PUBREL",
	7:  "PUBCOMP",
	8:  "SUBSCRIBE",
	9:  "SUBACK",
	10: "UNSUBSCRIBE",
	11: "UNSUBACK",
	12: "PINGREQ",
	13: "PINGRESP",
	14: "DISCONNECT",
}

const (
	Accepted                        = 0x00
	ErrRefusedBadProtocolVersion    = 0x01
	ErrRefusedIDRejected            = 0x02
	ErrRefusedServerUnavailable     = 0x03
	ErrRefusedBadUsernameOrPassword = 0x04
	ErrRefusedNotAuthorised         = 0x05
	ErrNetworkError                 = 0xFE
	ErrProtocolViolation            = 0xFF
)

var ConnackReturnCodes = map[uint8]string{
	0:   "Connection Accepted",
	1:   "Connection Refused: Bad Protocol Version",
	2:   "Connection Refused: Client Identifier Rejected",
	3:   "Connection Refused: Server Unavailable",
	4:   "Connection Refused: Username or Password in unknown format",
	5:   "Connection Refused: Not Authorised",
	254: "Connection Error",
	255: "Connection Refused: Protocol Violation",
}

var ConnErrors = map[byte]error{
	Accepted:                        nil,
	ErrRefusedBadProtocolVersion:    errors.New("Unnacceptable protocol version"),
	ErrRefusedIDRejected:            errors.New("Identifier rejected"),
	ErrRefusedServerUnavailable:     errors.New("Server Unavailable"),
	ErrRefusedBadUsernameOrPassword: errors.New("Bad user name or password"),
	ErrRefusedNotAuthorised:         errors.New("Not Authorized"),
	ErrNetworkError:                 errors.New("Network Error"),
	ErrProtocolViolation:            errors.New("Protocol Violation"),
}

func decodeByte(r io.Reader) (byte, error) {
	num := make([]byte, 1)
	_, err := r.Read(num)
	if err != nil {
		return 0, err
	}
	return num[0], nil
}

func decodeUint16(r io.Reader) (uint16, error) {
	num := make([]byte, 2)
	_, err := r.Read(num)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(num), nil
}

func encodeUint16(num uint16) []byte {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, num)
	return bytes
}

func encodeString(field string) []byte {
	fieldLength := make([]byte, 2)
	binary.BigEndian.PutUint16(fieldLength, uint16(len(field)))
	return append(fieldLength, []byte(field)...)
}

func decodeString(r io.Reader) (string, error) {
	fieldLength, err := decodeUint16(r)
	if err != nil {
		return "", err
	}

	field := make([]byte, fieldLength)
	_, err = r.Read(field)
	if err != nil {
		return "", err
	}
	return string(field), nil
}

func decodeBytes(r io.Reader) ([]byte, error) {
	fieldLength, err := decodeUint16(r)
	if err != nil {
		return nil, err
	}
	field := make([]byte, fieldLength)
	_, err = r.Read(field)
	if err != nil {
		return nil, err
	}
	return field, nil
}

func encodeBytes(field []byte) []byte {
	fieldLength := make([]byte, 2)
	binary.BigEndian.PutUint16(fieldLength, uint16(len(field)))
	return append(fieldLength, field...)
}

func encodeLength(length int) []byte {
	var encLength []byte
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		encLength = append(encLength, digit)
		if length == 0 {
			break
		}
	}
	return encLength
}

func decodeLength(r io.Reader) (int, error) {
	var rLength uint32
	var multiplier uint32
	b := make([]byte, 1)
	for {
		_, err := io.ReadFull(r, b)
		if err != nil {
			return 0, err
		}
		digit := b[0]
		rLength |= uint32(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}
	return int(rLength), nil
}

type FixedHeader struct {
	PacketType      byte
	Dup             bool
	Qos             byte
	Retain          bool
	RemainingLength int
}

func (fh FixedHeader) String() string {
	return fmt.Sprintf("%s: dup: %t qos: %d retain: %t rLength: %d", PacketNames[fh.PacketType], fh.Dup, fh.Qos, fh.Retain, fh.RemainingLength)
}

func boolToByte(b bool) byte {
	switch b {
	case true:
		return 1
	default:
		return 0
	}
}

func (fh *FixedHeader) pack() bytes.Buffer {
	var header bytes.Buffer
	header.WriteByte(fh.PacketType<<4 | boolToByte(fh.Dup)<<3 | fh.Qos<<1 | boolToByte(fh.Retain))
	header.Write(encodeLength(fh.RemainingLength))
	return header
}

func (fh *FixedHeader) unpack(typeAndFlags byte, r io.Reader) error {
	var err error
	fh.PacketType = typeAndFlags >> 4
	fh.Dup = (typeAndFlags>>3)&0x01 > 0
	fh.Qos = (typeAndFlags >> 1) & 0x03
	fh.Retain = typeAndFlags&0x01 > 0
	fh.RemainingLength, err = decodeLength(r)
	return err
}

func NewControlPacketWithHeader(fh FixedHeader) (cp ControlPacket) {
	switch fh.PacketType {
	case Connect:
		cp = &ConnectPacket{FixedHeader: fh}
	case Connack:
		cp = &ConnackPacket{FixedHeader: fh}
	case Disconnect:
		cp = &DisconnectPacket{FixedHeader: fh}
	case Publish:
		cp = &PublishPacket{FixedHeader: fh}
	case Puback:
		cp = &PubackPacket{FixedHeader: fh}
	case Pubrec:
		cp = &PubrecPacket{FixedHeader: fh}
	case Pubrel:
		cp = &PubrelPacket{FixedHeader: fh}
	case Pubcomp:
		cp = &PubcompPacket{FixedHeader: fh}
	case Subscribe:
		cp = &SubscribePacket{FixedHeader: fh}
	case Suback:
		cp = &SubackPacket{FixedHeader: fh}
	case Unsubscribe:
		cp = &UnsubscribePacket{FixedHeader: fh}
	case Unsuback:
		cp = &UnsubackPacket{FixedHeader: fh}
	case Pingreq:
		cp = &PingreqPacket{FixedHeader: fh}
	case Pingresp:
		cp = &PingrespPacket{FixedHeader: fh}
	default:
		return nil
	}
	return cp
}

func NewControlPacket(packetType byte) (cp ControlPacket) {
	switch packetType {
	case Connect:
		cp = &ConnectPacket{FixedHeader: FixedHeader{PacketType: Connect}}
	case Connack:
		cp = &ConnackPacket{FixedHeader: FixedHeader{PacketType: Connack}}
	case Disconnect:
		cp = &DisconnectPacket{FixedHeader: FixedHeader{PacketType: Disconnect}}
	case Publish:
		cp = &PublishPacket{FixedHeader: FixedHeader{PacketType: Publish}}
	case Puback:
		cp = &PubackPacket{FixedHeader: FixedHeader{PacketType: Puback}}
	case Pubrec:
		cp = &PubrecPacket{FixedHeader: FixedHeader{PacketType: Pubrec}}
	case Pubrel:
		cp = &PubrelPacket{FixedHeader: FixedHeader{PacketType: Pubrel, Qos: 1}}
	case Pubcomp:
		cp = &PubcompPacket{FixedHeader: FixedHeader{PacketType: Pubcomp}}
	case Subscribe:
		cp = &SubscribePacket{FixedHeader: FixedHeader{PacketType: Subscribe, Qos: 1}}
	case Suback:
		cp = &SubackPacket{FixedHeader: FixedHeader{PacketType: Suback}}
	case Unsubscribe:
		cp = &UnsubscribePacket{FixedHeader: FixedHeader{PacketType: Unsubscribe, Qos: 1}}
	case Unsuback:
		cp = &UnsubackPacket{FixedHeader: FixedHeader{PacketType: Unsuback}}
	case Pingreq:
		cp = &PingreqPacket{FixedHeader: FixedHeader{PacketType: Pingreq}}
	case Pingresp:
		cp = &PingrespPacket{FixedHeader: FixedHeader{PacketType: Pingresp}}
	default:
		return nil
	}
	return cp
}

func ReadPacket(r io.Reader) (cp ControlPacket, err error) {
	var fh FixedHeader
	b := make([]byte, 1)

	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	fh.unpack(b[0], r)
	cp = NewControlPacketWithHeader(fh)
	if cp == nil {
		return nil, errors.New("Bad data from client")
	}
	packetBytes := make([]byte, fh.RemainingLength)
	_, err = io.ReadFull(r, packetBytes)
	if err != nil {
		return nil, err
	}
	err = cp.ReadFrom(bytes.NewBuffer(packetBytes))
	return cp, err
}
