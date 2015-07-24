package broker

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/zwczou/mqtt/packets"
)

// A retain holds information necessary to correctly manage retained
// messages.
//
// This needs to hold copies of the proto.Publish, not pointers to
// it, or else we can send out one with the wrong retain flag.
type retain struct {
	m    *packets.PublishPacket
	wild wild
}

// A post is a unit of work for the subscription processing workers.
type post struct {
	c *incomingConn
	m *packets.PublishPacket
}

type subscriptions struct {
	workers int
	posts   chan (post)

	mu        sync.Mutex // guards access to fields below
	subs      map[string][]*incomingConn
	wildcards []wild
	retain    map[string]retain
	stats     *stats
}

// The length of the queue that subscription processing
// workers are taking from.
const postQueue = 100

func newSubscriptions(workers int) *subscriptions {
	s := &subscriptions{
		subs:    make(map[string][]*incomingConn),
		retain:  make(map[string]retain),
		posts:   make(chan post, postQueue),
		workers: workers,
	}
	for i := 0; i < s.workers; i++ {
		go s.run(i)
	}
	return s
}

func (s *subscriptions) sendRetain(topic string, c *incomingConn) {
	s.mu.Lock()
	var tlist []string
	if isWildcard(topic) {

		// TODO: select matching topics from the retain map
	} else {
		tlist = []string{topic}
	}
	for _, t := range tlist {
		if r, ok := s.retain[t]; ok {
			c.submit(r.m)
		}
	}
	s.mu.Unlock()
}

func (s *subscriptions) add(topic string, c *incomingConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if isWildcard(topic) {
		w := newWild(topic, c)
		if w.valid() {
			s.wildcards = append(s.wildcards, w)
		}
	} else {
		s.subs[topic] = append(s.subs[topic], c)
	}
}

// Find all connections that are subscribed to this topic.
func (s *subscriptions) subscribers(topic string) []*incomingConn {
	s.mu.Lock()
	defer s.mu.Unlock()

	// non-wildcard subscribers
	res := s.subs[topic]

	// process wildcards
	parts := strings.Split(topic, "/")
	for _, w := range s.wildcards {
		if w.matches(parts) {
			res = append(res, w.c)
		}
	}

	return res
}

// Remove all subscriptions that refer to a connection.
func (s *subscriptions) unsubAll(c *incomingConn) {
	s.mu.Lock()
	for _, v := range s.subs {
		for i := range v {
			if v[i] == c {
				v[i] = nil
			}
		}
	}

	// remove any associated entries in the wildcard list
	var wildNew []wild
	for i := 0; i < len(s.wildcards); i++ {
		if s.wildcards[i].c != c {
			wildNew = append(wildNew, s.wildcards[i])
		}
	}
	s.wildcards = wildNew

	s.mu.Unlock()
}

// Remove the subscription to topic for a given connection.
func (s *subscriptions) unsub(topic string, c *incomingConn) {
	s.mu.Lock()
	if subs, ok := s.subs[topic]; ok {
		nils := 0

		// Search the list, removing references to our connection.
		// At the same time, count the nils to see if this list is now empty.
		for i := 0; i < len(subs); i++ {
			if subs[i] == c {
				subs[i] = nil
			}
			if subs[i] == nil {
				nils++
			}
		}

		if nils == len(subs) {
			delete(s.subs, topic)
		}
	}
	s.mu.Unlock()
}

// The subscription processing worker.
func (s *subscriptions) run(id int) {
	tag := fmt.Sprintf("worker %d ", id)
	log.Print(tag, "started")
	for post := range s.posts {
		// Remember the original retain setting, but send out immediate
		// copies without retain: "When a server sends a PUBLISH to a client
		// as a result of a subscription that already existed when the
		// original PUBLISH arrived, the Retain flag should not be set,
		// regardless of the Retain flag of the original PUBLISH.
		isRetain := post.m.Retain
		post.m.Retain = false

		// Handle "retain with payload size zero = delete retain".
		// Once the delete is done, return instead of continuing.
		if isRetain && len(post.m.Payload) == 0 {
			s.mu.Lock()
			delete(s.retain, post.m.TopicName)
			s.mu.Unlock()
			return
		}

		// Find all the connections that should be notified of this message.
		conns := s.subscribers(post.m.TopicName)

		// Queue the outgoing messages
		for _, c := range conns {
			if c != nil {
				c.submit(post.m)
			}
		}

		if isRetain {
			s.mu.Lock()
			// Save a copy of it, and set that copy's Retain to true, so that
			// when we send it out later we notify new subscribers that this
			// is an old message.
			msg := post.m
			msg.Retain = true
			s.retain[post.m.TopicName] = retain{m: msg}
			s.mu.Unlock()
		}
	}
}

func (s *subscriptions) submit(c *incomingConn, m *packets.PublishPacket) {
	s.posts <- post{c: c, m: m}
}
