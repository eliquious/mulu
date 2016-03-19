package server

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"

	"github.com/coocood/freecache"
	"golang.org/x/net/context"
)

// const RingBufferCapacity = 4 * 1024 * 1024
// const RingBufferMask = RingBufferCapacity - 1

func NewTcpHandler(cache *freecache.Cache, conn net.Conn, logger *log.Logger, ctx context.Context) *tcpHandler {
	w := NewFixedSizeWriter(conn, 64*1024)
	c, cancel := context.WithCancel(ctx)
	return &tcpHandler{cache,
		logger, conn, w, &Parser{logger: logger, writer: w, cache: cache}, c, cancel,
	}
}

type tcpHandler struct {
	cache   *freecache.Cache
	logger  *log.Logger
	conn    net.Conn
	writer  *FixedSizeWriter
	parser  *Parser
	context context.Context
	cancel  context.CancelFunc
}

func (t *tcpHandler) Execute() {
	defer t.conn.Close()

	// Read from connection
	go t.createReadLoop()
	<-t.context.Done()

	t.logger.Printf("Closing connection: %+v", t.conn.RemoteAddr())
	t.writer.Flush()
}

func (t *tcpHandler) createReadLoop() {
	defer t.cancel()

	inputChan, _ := createScanner(t.conn)

	// OUTER:
	for {
		select {
		case <-t.context.Done():
			t.writer.Flush()
			return
		case input := <-inputChan:
			if input == nil {
				return
			}

			sent := t.parser.Parse(bytes.Trim(input, "\r\n"))
			if !sent {
				t.logger.Printf("Parse failed: %+v", string(input))
				return
			}

			// case <-time.After(time.Second):
			t.writer.Flush()

			// case err := <-errChan:
			// 	if err == nil {
			// 		t.writer.Write(err)
			// 		t.logger.Printf("Read failure: %+v", string(err))
			// 		return
			// 	}
		}
	}

	// buffer := make([]byte, 256*1024)
	// parseBuffer := make([]byte, 1024)
	// var idx, offset int
	// for {
	// 	select {
	// 	case <-t.context.Done():
	// 		return
	// 	default:
	// 		n, err := t.conn.Read(buffer)
	// 		if n > 0 {
	// 			for idx = 0; idx < n; idx++ {
	// 				if offset >= len(parseBuffer) {
	// 					t.writer.Write(ErrMaxSize)
	// 					t.logger.Printf("ERR %s", string(ErrMaxSize))
	// 					return
	// 				}
	//
	// 				// end of request
	// 				if parseBuffer[offset] == '\n' {
	// 					t.parser.Parse(parseBuffer[:offset])
	//
	// 					// reset request size to 0
	// 					offset = 0
	// 				} else if parseBuffer[offset] == '\r' {
	// 					continue
	// 				} else {
	// 					parseBuffer[offset] = buffer[idx]
	// 					offset++
	// 				}
	// 			}
	// 		} else if err == io.EOF {
	// 			return
	// 		}
	// 		if err != nil && err != io.EOF {
	// 			return
	// 		}
	// 	}
	// }
}

func createScanner(r io.Reader) (chan []byte, chan []byte) {
	c := make(chan []byte, 1)
	e := make(chan []byte, 1)
	go func(input io.Reader) {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			c <- scanner.Bytes()
		}
		if err := scanner.Err(); err != nil {
			if err == bufio.ErrTooLong || err == bufio.ErrBufferFull {
				e <- ErrMaxSize
			} else {
				e <- ErrReadError
			}
		}
		close(c)
		close(e)
	}(r)
	return c, e
}
