package broker

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/zwczou/mqtt/packets"
)

// An IncomingConn represents a connection into a Server.
type incomingConn struct {
	svr            *Server
	conn           net.Conn
	jobs           chan job
	clientid       string
	connect        *packets.ConnectPacket
	KeepaliveTimer uint16
	Done           chan struct{}
	stop           chan struct{}
}

var clients = make(map[string]*incomingConn)
var clientsMu sync.Mutex

const sendingQueueLength = 20

// newIncomingConn creates a new incomingConn associated with this
// server. The connection becomes the property of the incomingConn
// and should not be touched again by the caller until the Done
// channel becomes readable.
func (s *Server) newIncomingConn(conn net.Conn) *incomingConn {
	return &incomingConn{
		svr:  s,
		conn: conn,
		jobs: make(chan job, sendingQueueLength),
		Done: make(chan struct{}),
		stop: make(chan struct{}),
	}
}

type receipt chan struct{}

// Wait for the receipt to indicate that the job is done.
func (r receipt) wait() {
	<-r
}

func (r receipt) waitTimeout(t time.Duration) {
	select {
	case <-r:
	case <-time.After(t):
	}
}

type job struct {
	m packets.ControlPacket
	r receipt
}

// Start reading and writing on this connection.
func (c *incomingConn) start() {
	go c.reader()
	go c.writer()
}

// Add this connection to the map, or find out that an existing connection
// already exists for the same client-id.
func (c *incomingConn) add() *incomingConn {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	existing, ok := clients[c.clientid]
	if !ok {
		// this client id already exists, return it
		return existing
	}

	clients[c.clientid] = c
	return nil
}

// Delete a connection; the conection must be closed by the caller first.
func (c *incomingConn) del() {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients, c.clientid)
	return
}

// Replace any existing connection with this one. The one to be replaced,
// if any, must be closed first by the caller.
func (c *incomingConn) replace() {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	// Check that any existing connection is already closed.
	existing, ok := clients[c.clientid]
	if ok {
		die := false
		select {
		case _, ok := <-existing.jobs:
			// what? we are expecting that this channel is closed!
			if ok {
				die = true
			}
		default:
			die = true
		}
		if die {
			panic("attempting to replace a connection that is not closed")
		}

		delete(clients, c.clientid)
	}

	clients[c.clientid] = c
	return
}

// Queue a message; no notification of sending is done.
func (c *incomingConn) submit(m packets.ControlPacket) {
	j := job{m: m}
	select {
	case c.jobs <- j:
	default:
		log.Print(c, ": failed to submit message")
	}
	return
}

func (c *incomingConn) String() string {
	return fmt.Sprintf("{IncomingConn: %v}", c.clientid)
}

// Queue a message, returns a channel that will be readable
// when the message is sent.
func (c *incomingConn) submitSync(m packets.ControlPacket) receipt {
	j := job{m: m, r: make(receipt)}
	c.jobs <- j
	return j.r
}

func (c *incomingConn) reader() {
	var err error
	var zeroTime time.Time
	var m packets.ControlPacket

	for {
		if c.KeepaliveTimer > 0 {
			c.conn.SetReadDeadline(time.Now().Add(time.Duration(c.KeepaliveTimer) * time.Second))
		} else {
			c.conn.SetReadDeadline(zeroTime)
		}

		m, err = packets.ReadPacket(c.conn)
		if err != nil {
			break
		}
		c.svr.stats.messageRecv()

		if c.svr.Dump {
			log.Printf("dump  in: %T", m)
		}

		switch m := m.(type) {
		case *packets.ConnectPacket:
			rc := m.Validate()
			if rc != packets.Accepted {
				err = packets.ConnErrors[rc]
				log.Printf("Connection refused for %v: %v", c.conn.RemoteAddr(), ConnectionErrors[rc])
				goto exit
			}

			// connack
			connack := packets.NewControlPacket(packets.Connack)
			connack.(*packets.ConnackPacket).ReturnCode = rc
			c.submit(connack)

			c.clientid = m.ClientIdentifier
			c.KeepaliveTimer = m.KeepaliveTimer
			c.connect = m

			// Disconnect existing connections.
			if existing := c.add(); existing != nil {
				disconnect := packets.NewControlPacket(packets.Disconnect)
				r := existing.submitSync(disconnect)
				r.waitTimeout(time.Second)
			}
			c.add()

			// Log in mosquitto format.
			clean := 0
			if m.CleanSession {
				clean = 1
			}
			log.Printf("New client connected from %v as %v (c%v, k%v).", c.conn.RemoteAddr(), c.clientid, clean, m.KeepaliveTimer)

		case *packets.PublishPacket:
			switch m.Qos {
			case 2:
				pr := packets.NewControlPacket(packets.Pubrec).(*packets.PubrecPacket)
				pr.PacketID = m.PacketID
				c.submit(pr)
				c.svr.subs.submit(c, m)
			case 1:
				c.svr.subs.submit(c, m)

				pa := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
				pa.PacketID = m.PacketID
				c.submit(pa)
			case 0:
				c.svr.subs.submit(c, m)
			}
		case *packets.PubackPacket:
			// TODO: free message
		case *packets.PubrecPacket:
			prel := packets.NewControlPacket(packets.Pubrel).(*packets.PubrelPacket)
			prel.PacketID = m.PacketID
			c.submit(prel)
		case *packets.PubrelPacket:
			pc := packets.NewControlPacket(packets.Pubcomp).(*packets.PubcompPacket)
			pc.PacketID = m.PacketID
			c.submit(pc)
		case *packets.PubcompPacket:
			// TODO: free message
		case *packets.PingreqPacket:
			pr := packets.NewControlPacket(packets.Pingresp)
			c.submit(pr)
		case *packets.SubscribePacket:
			for i := 0; i < len(m.Topics); i++ {
				topic := m.Topics[i]
				c.svr.subs.add(topic, c)
			}

			suback := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
			suback.GrantedQoss = m.Qoss
			c.submit(suback)

			for _, topic := range m.Topics {
				c.svr.subs.sendRetain(topic, c)
			}
		case *packets.UnsubscribePacket:
			unsuback := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
			unsuback.PacketID = m.PacketID
			for _, t := range m.Topics {
				c.svr.subs.unsub(t, c)
			}
			c.submit(unsuback)

		case *packets.DisconnectPacket:
			goto exit
		default:
			err = fmt.Errorf("unknown msg type %T", m)
			goto exit
		}
	}

exit:
	if err != nil {
		log.Printf("reader error: %s", err)
		// TODO: last will

		if c.connect.WillFlag {
			pub := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
			pub.Qos = c.connect.WillQos
			pub.Retain = c.connect.WillRetain
			pub.TopicName = c.connect.WillTopic
			pub.Payload = c.connect.WillMessage
			c.svr.subs.submit(c, pub)
		}
	}

	c.del()
	c.svr.subs.unsubAll(c)
	c.svr.stats.clientDisconnect()
	close(c.stop)
	c.conn.Close()
}

func (c *incomingConn) writer() {
	var err error

	for {
		select {
		case job := <-c.jobs:
			err = job.m.WriteTo(c.conn)
			if job.r != nil {
				close(job.r)
			}
			if err != nil {
				goto exit
			}
			if _, ok := job.m.(*packets.DisconnectPacket); ok {
				goto exit
			}
			c.svr.stats.messageSend()
		case <-c.stop:
			goto exit
		}
	}

exit:
}
