package broker

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/zwczou/mqtt/packets"
)

type stats struct {
	recv       int64
	sent       int64
	clients    int64
	clientsMax int64
	lastmsgs   int64
}

func (s *stats) messageRecv()      { atomic.AddInt64(&s.recv, 1) }
func (s *stats) messageSend()      { atomic.AddInt64(&s.sent, 1) }
func (s *stats) clientConnect()    { atomic.AddInt64(&s.clients, 1) }
func (s *stats) clientDisconnect() { atomic.AddInt64(&s.clients, -1) }

func statsMessage(topic string, stat int64) *packets.PublishPacket {
	p := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	p.Qos = 1
	p.Retain = true
	p.Dup = false
	p.TopicName = topic
	p.Payload = []byte(fmt.Sprintf("%v", stat))
	return p
}

func (s *stats) publish(sub *subscriptions, interval time.Duration) {
	clients := atomic.LoadInt64(&s.clients)
	clientsMax := atomic.LoadInt64(&s.clientsMax)
	if clients > clientsMax {
		clientsMax = clients
		atomic.StoreInt64(&s.clientsMax, clientsMax)
	}
	sub.submit(nil, statsMessage("$SYS/broker/clients/active", clients))
	sub.submit(nil, statsMessage("$SYS/broker/clients/maximum", clientsMax))
	sub.submit(nil, statsMessage("$SYS/broker/messages/received",
		atomic.LoadInt64(&s.recv)))
	sub.submit(nil, statsMessage("$SYS/broker/messages/sent",
		atomic.LoadInt64(&s.sent)))

	msgs := atomic.LoadInt64(&s.recv) + atomic.LoadInt64(&s.sent)
	msgpersec := (msgs - s.lastmsgs) / int64(interval/time.Second)
	// no need for atomic because we are the only reader/writer of it
	s.lastmsgs = msgs

	sub.submit(nil, statsMessage("$SYS/broker/messages/per-sec", msgpersec))
}
