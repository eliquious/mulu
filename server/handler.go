package server

import (
	"io"
	"log"
	"net"

	"github.com/coocood/freecache"
	disruptor "github.com/smartystreets/go-disruptor"
	"golang.org/x/net/context"
)

const RingBufferCapacity = 4 * 1024 * 1024
const RingBufferMask = RingBufferCapacity - 1

func NewTcpHandler(cache *freecache.Cache, conn net.Conn, logger *log.Logger, ctx context.Context) *tcpHandler {
	ring := [RingBufferCapacity]byte{}
	w := NewFixedSizeWriter(conn, 256*1024)
	controller := disruptor.
		Configure(RingBufferCapacity).
		WithConsumerGroup(&ByteConsumer{
		Writer: w,
		Closer: conn,
		Parser: &Parser{logger: logger, writer: w, cache: cache},
		ring:   &ring,
		cache:  cache,
		logger: logger,
	}).Build()
	controller.Start()

	c, cancel := context.WithCancel(ctx)
	return &tcpHandler{cache,
		logger, conn, &ring, &controller, c, cancel,
	}
}

type tcpHandler struct {
	cache      *freecache.Cache
	logger     *log.Logger
	conn       net.Conn
	ring       *[RingBufferCapacity]byte
	controller *disruptor.Disruptor
	context    context.Context
	cancel     context.CancelFunc
}

func (t *tcpHandler) Execute() {
	defer t.conn.Close()
	defer t.controller.Stop()

	// Read from connection
	go t.createReadLoop()
	<-t.context.Done()
}

func (t *tcpHandler) createReadLoop() {
	defer t.cancel()
	writer := t.controller.Writer()
	buffer := make([]byte, 256*1024)
	var sequence, reservations int64
	var idx int
	for {
		select {
		case <-t.context.Done():
			return
		default:
			n, err := t.conn.Read(buffer)
			if n > 0 {
				idx = 0
				reservations = int64(n)
				sequence = writer.Reserve(reservations)
				for lower := sequence - reservations + 1; lower <= sequence; lower++ {
					t.ring[lower&RingBufferMask] = buffer[idx]
					idx++
				}
				writer.Commit(sequence-reservations+1, sequence)
			} else if err == io.EOF {
				return
			}
			if err != nil && err != io.EOF {
				return
			}
		}
	}
}
