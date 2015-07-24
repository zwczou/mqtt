package broker

import (
	"errors"
	"log"
	"math/rand"
	"net"
	"runtime"
	"time"

	proto "github.com/huin/mqtt"
)

var ConnectionErrors = [6]error{
	nil, // Connection Accepted (not an error)
	errors.New("Connection Refused: unacceptable protocol version"),
	errors.New("Connection Refused: identifier rejected"),
	errors.New("Connection Refused: server unavailable"),
	errors.New("Connection Refused: bad user name or password"),
	errors.New("Connection Refused: not authorized"),
}

// header is used to initialize a proto.Header when the zero value
// is not correct. The zero value of proto.Header is
// the equivalent of header(dupFalse, proto.QosAtMostOnce, retainFalse)
// and is correct for most messages.
func header(d dupFlag, q proto.QosLevel, r retainFlag) proto.Header {
	return proto.Header{
		DupFlag: bool(d), QosLevel: q, Retain: bool(r),
	}
}

type retainFlag bool
type dupFlag bool

const (
	retainFalse retainFlag = false
	retainTrue             = true
	dupFalse    dupFlag    = false
	dupTrue                = true
)

const clientQueueLength = 100

// A Server holds all the state associated with an MQTT server.
type Server struct {
	l             net.Listener
	subs          *subscriptions
	stats         *stats
	Done          chan struct{}
	StatsInterval time.Duration // Defaults to 10 seconds. Must be set using sync/atomic.StoreInt64().
	Dump          bool          // When true, dump the messages in and out.
	rand          *rand.Rand
}

// NewServer creates a new MQTT server, which accepts connections from
// the given listener. When the server is stopped (for instance by
// another goroutine closing the net.Listener), channel Done will become
// readable.
func NewServer(l net.Listener) *Server {
	svr := &Server{
		l:             l,
		stats:         &stats{},
		Done:          make(chan struct{}),
		StatsInterval: time.Second * 10,
		subs:          newSubscriptions(runtime.GOMAXPROCS(0)),
	}

	// start the stats reporting goroutine
	go func() {
		for {
			svr.stats.publish(svr.subs, svr.StatsInterval)
			select {
			case <-svr.Done:
				return
			default:
				// keep going
			}
			time.Sleep(svr.StatsInterval)
		}
	}()

	return svr
}

// Start makes the Server start accepting and handling connections.
func (s *Server) Start() {
	go func() {
		for {
			conn, err := s.l.Accept()
			if err != nil {
				log.Print("Accept: ", err)
				break
			}

			cli := s.newIncomingConn(conn)
			s.stats.clientConnect()
			cli.start()
		}
		close(s.Done)
	}()
}
