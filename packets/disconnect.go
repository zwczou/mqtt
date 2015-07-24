package packets

import (
	"fmt"
	"io"
)

type DisconnectPacket struct {
	FixedHeader
}

func (d *DisconnectPacket) String() string {
	str := fmt.Sprintf("%s\n", d.FixedHeader)
	return str
}

func (d *DisconnectPacket) WriteTo(w io.Writer) error {
	packet := d.FixedHeader.pack()
	_, err := packet.WriteTo(w)

	return err
}

func (d *DisconnectPacket) ReadFrom(r io.Reader) error {
	return nil
}

func (d *DisconnectPacket) Details() Details {
	return Details{Qos: 0, PacketID: 0}
}
