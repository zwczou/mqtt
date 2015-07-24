package packets

import (
	"fmt"
	"io"
)

type PingrespPacket struct {
	FixedHeader
}

func (pr *PingrespPacket) String() string {
	str := fmt.Sprintf("%s", pr.FixedHeader)
	return str
}

func (pr *PingrespPacket) WriteTo(w io.Writer) error {
	packet := pr.FixedHeader.pack()
	_, err := packet.WriteTo(w)
	return err
}

func (pr *PingrespPacket) ReadFrom(r io.Reader) error {
	return nil
}

func (pr *PingrespPacket) Details() Details {
	return Details{Qos: 0, PacketID: 0}
}
