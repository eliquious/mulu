package server

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/coocood/freecache"
	"golang.org/x/net/context"
)

func NewServer(cache *freecache.Cache, logger *log.Logger) *Server {
	return &Server{
		cache:  cache,
		logger: logger,
	}
}

func NewServerSize(cachesize int, logger *log.Logger) *Server {
	return &Server{
		cache:  freecache.NewCache(0),
		logger: logger,
	}
}

// Server handles all the incoming connections as well as handler dispatch.
type Server struct {
	cache    *freecache.Cache
	logger   *log.Logger
	addr     *net.TCPAddr
	listener *net.TCPListener
	context  context.Context
	cancel   context.CancelFunc
}

// Start starts accepting client connections. This method is non-blocking.
func (s *Server) Start(addr string) (err error) {
	// Validate the ssh bind addr
	if addr == "" {
		err = fmt.Errorf("server: Empty bind address")
		return
	}

	// Open SSH socket listener
	netAddr, e := net.ResolveTCPAddr("tcp", addr)
	if e != nil {
		err = fmt.Errorf("server: Invalid tcp address")
		return
	}

	// Create listener
	listener, e := net.ListenTCP("tcp", netAddr)
	if e != nil {
		err = e
		return
	}

	s.listener = listener
	s.addr = listener.Addr().(*net.TCPAddr)
	s.logger.Println("Starting server", "addr", addr)

	c, cancel := context.WithCancel(context.Background())
	s.context = c
	s.cancel = cancel
	go s.listen(c)

	<-c.Done()
	return
}

// Stop stops the server and kills all goroutines. This method is blocking.
func (s *Server) Stop() {
	s.logger.Println("[INFO] Shutting down server...")
	s.cancel()
}

// listen accepts new connections and handles the conversion from TCP to SSH connections.
func (s *Server) listen(c context.Context) {
	defer s.listener.Close()

	for {
		// Accepts will only block for 1s
		s.listener.SetDeadline(time.Now().Add(time.Second))

		select {

		// Stop server on channel receive
		case <-c.Done():
			s.logger.Println("[DEBUG] Context Completed")
			return
		default:

			// Accept new connection
			tcpConn, err := s.listener.Accept()
			if err != nil {
				if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
					// s.Logger.Println("[DBG] Connection timeout...")
				} else {
					s.logger.Println("[WRN] Connection failed", "error", err)
				}
				continue
			}

			// Handle connection
			s.logger.Println("[INF] Successful TCP connection:", tcpConn.RemoteAddr().String())
			h := NewTcpHandler(s.cache, tcpConn, s.logger, s.context)
			go h.Execute()
		}
	}
}

// type ByteConsumer struct {
// 	Writer FlushableWriter
// 	Closer io.Closer
// 	Parser *Parser
// 	logger *log.Logger
// 	cache  *freecache.Cache
// 	ring   *[RingBufferCapacity]byte
// 	buffer [65336]byte
// 	// closed      bool
// 	requestSize int
// }
//
// func (b *ByteConsumer) Consume(lower, upper int64) {
// 	defer b.Writer.Flush()
//
// 	var char byte
// 	for sequence := lower; sequence <= upper; sequence++ {
// 		if b.requestSize >= len(b.buffer) {
// 			b.Writer.Write(ErrMaxSize)
// 			b.logger.Printf("ERR %s\r\n", string(ErrMaxSize))
// 			return
// 		}
// 		char = b.ring[sequence&RingBufferMask]
//
// 		// end of request
// 		if char == '\n' {
// 			_ = b.Parser.Parse(b.buffer[:b.requestSize])
//
// 			// reset request size to 0
// 			b.requestSize = 0
// 		} else if char == '\r' {
// 			continue
// 		} else {
// 			b.buffer[b.requestSize] = char
// 			b.requestSize++
// 		}
// 	}
// }
