package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/coocood/freecache"
	"golang.org/x/net/context"
)

// Server handles all the incoming connections as well as handler dispatch.
type Server struct {
	Cache    *freecache.Cache
	Logger   *log.Logger
	Addr     *net.TCPAddr
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

	s.Logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
	s.listener = listener
	s.Addr = listener.Addr().(*net.TCPAddr)
	s.Logger.Println("Starting server", "addr", addr)

	c, cancel := context.WithCancel(context.Background())
	s.context = c
	s.cancel = cancel
	go s.listen(c)

	<-c.Done()
	return
}

// Stop stops the server and kills all goroutines. This method is blocking.
func (s *Server) Stop() {
	s.Logger.Println("[INFO] Shutting down server...")
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
			s.Logger.Println("[DEBUG] Context Completed")
			return
		default:

			// Accept new connection
			tcpConn, err := s.listener.Accept()
			if err != nil {
				if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
					// s.Logger.Println("[DBG] Connection timeout...")
				} else {
					s.Logger.Println("[WRN] Connection failed", "error", err)
				}
				continue
			}

			// Handle connection
			s.Logger.Println("[INF] Successful TCP connection:", tcpConn.RemoteAddr().String())
			h := NewTcpHandler(s.Cache, tcpConn, s.Logger, s.context)
			go h.Execute()
		}
	}
}

type ByteConsumer struct {
	Writer      *bufio.Writer
	Closer      io.Closer
	Parser      *Parser
	logger      *log.Logger
	cache       *freecache.Cache
	ring        *[RingBufferCapacity]byte
	buffer      [65336 * 4]byte
	closed      bool
	requestSize int
}

func (b *ByteConsumer) Consume(lower, upper int64) {
	if b.closed {
		return
	}
	defer b.Writer.Flush()
	// b.logger.Printf("Consuming %d-%d\r\n", lower, upper)

	var char byte
	for sequence := lower; sequence <= upper; sequence++ {
		if b.requestSize >= len(b.buffer) {
			b.Writer.Write(ErrMaxSize)
			b.logger.Printf("ERR %s\r\n", string(ErrMaxSize))
			b.Writer.Flush()
			b.Closer.Close()
			b.closed = true
			return
		}
		char = b.ring[sequence&RingBufferMask]
		// b.logger.Printf("char '%s'\r\n", char)

		// end of request
		if char == '\n' {
			line := b.buffer[:b.requestSize]
			ok := b.Parser.Parse(line)
			b.closed = !ok
			if b.closed {
				b.Writer.Flush()
				b.Closer.Close()
				return
			}

			// reset request size to 0
			b.requestSize = 0

			// also skip the new line that follows the \r
			// sequence += 1
		} else if char == '\r' {
			continue
		} else {
			b.buffer[b.requestSize] = char
			b.requestSize++
		}
	}
}
