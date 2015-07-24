package broker

import (
	"log"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

// A Server holds all the state associated with an MQTT server.
type Server struct {
	sync.WaitGroup
	l               net.Listener
	subs            *subscriptions
	stats           *stats
	StatsInterval   time.Duration // Defaults to 10 seconds. Must be set using sync/atomic.StoreInt64().
	SendQueueLength int
	Dump            bool // When true, dump the messages in and out.
	stop            chan struct{}
}

// NewServer creates a new MQTT server, which accepts connections from
// the given listener. When the server is stopped (for instance by
// another goroutine closing the net.Listener), channel Stop will become
// readable.
func NewServer(l net.Listener) *Server {
	svr := &Server{
		l:               l,
		stats:           &stats{},
		stop:            make(chan struct{}),
		StatsInterval:   time.Second * 10,
		SendQueueLength: 20,
		subs:            newSubscriptions(runtime.NumCPU()),
	}

	// start the stats reporting goroutine
	go func() {
		ticker := time.NewTicker(svr.StatsInterval)
		for {
			select {
			case <-ticker.C:
				svr.stats.publish(svr.subs, svr.StatsInterval)
			case <-svr.stop:
				return
			}
		}
	}()

	return svr
}

// Start makes the Server start accepting and handling connections.
func (s *Server) Start() {
	s.Add(1)
	go func() {
		for {
			conn, err := s.l.Accept()
			if err != nil {
				if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
					log.Printf("NOTICE: temporary Accept() failure - %s", err)
					runtime.Gosched()
					continue
				}

				if strings.Contains(err.Error(), "use of closed") {
					break
				}

				log.Print("INFO: failed to accept -", err)
				break
			}

			cli := s.newIncomingConn(conn)
			s.stats.clientConnect()
			cli.start()
		}
		close(s.stop)
		s.Done()
	}()
}

func (s *Server) Stop() {
	close(s.subs.stop)
	s.subs.Wait()
	if s.l != nil {
		s.l.Close()
		s.l = nil
	}
	s.Wait()
}
