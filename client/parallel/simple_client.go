package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"
)

var requestCount uint64
var totalPingsPerConnection uint64 = 1000000
var concurrentConnections uint64 = 8
var totalPings = concurrentConnections * totalPingsPerConnection

func main() {
	runtime.GOMAXPROCS(4)

	done := make(chan bool, 1)
	exit := monitor(done)

	start := time.Now()
	var wg sync.WaitGroup
	for i := uint64(0); i < concurrentConnections; i++ {
		wg.Add(1)
		go client(&wg)
	}

	wg.Wait()
	done <- true
	<-exit
	duration := time.Now().Sub(start)
	fmt.Println(duration)
	fmt.Println(duration / time.Duration(totalPings))

}

func client(wg *sync.WaitGroup) {
	defer wg.Done()

	outBuf := []byte(fmt.Sprintf("GET key%d\r\n", rand.Intn(8)))
	servAddr := "localhost:9022"
	tcpAddr, err := net.ResolveTCPAddr("tcp", servAddr)
	if err != nil {
		println("ResolveTCPAddr failed:", err.Error())
		os.Exit(1)
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		println("Dial failed:", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	// Create write loop
	ctx, cancel := context.WithCancel(context.Background())
	writeChan := createWriteLoop(ctx, conn, outBuf)
	readChan := createReadLoop(ctx, conn)

	// wr := bufio.NewWriterSize(conn, 64*1024)
	// wr := bio.NewFixedSizeWriter(conn, 1024*1024)
	for {

		select {
		case <-time.After(time.Second):
			if atomic.LoadUint64(&requestCount) >= totalPings {
				cancel()
				return
			}
		case <-writeChan:
			return
		case <-readChan:
			return
		}
		// atomic.AddUint64(&requestCount, 1)
	}

	// wr := bufio.NewWriterSize(conn, 4*1024)
	// for atomic.LoadUint64(&requestCount) < totalPings {
	//
	// 	_, err = conn.Write(outBuf)
	// 	if err != nil {
	// 		println("Write to server failed:", err.Error())
	// 		os.Exit(1)
	// 	}
	//
	// 	// println("write to server = ", strEcho)
	//
	// 	reply := make([]byte, 1024)
	// 	_, err = conn.Read(reply)
	// 	if err != nil {
	// 		println("Write to server failed:", err.Error())
	// 		os.Exit(1)
	// 	}
	//
	// 	atomic.AddUint64(&requestCount, 1)
	// }
	// conn.Close()
}

func createReadLoop(ctx context.Context, r io.Reader) chan struct{} {
	c := make(chan struct{}, 1)
	go func() {
		defer close(c)
		bufr := bufio.NewReaderSize(r, 64*1024)
		scanner := bufio.NewScanner(bufr)
		for scanner.Scan() {
			// data = scanner.Bytes()
			select {
			case <-ctx.Done():
				return
			// case c <- &data:
			default:
				atomic.AddUint64(&requestCount, 1)
			}
		}
	}()
	return c
}

func createWriteLoop(ctx context.Context, w io.Writer, output []byte) chan struct{} {
	c := make(chan struct{}, 1)
	go func() {
		defer close(c)
		wr := bufio.NewWriterSize(w, 1024*1024)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				if wr.Buffered() > 0 {
					wr.Flush()
				}
			default:
				_, err := wr.Write(output)
				if err != nil {
					println("Write to server failed:", err.Error())
					// os.Exit(1)
					return
				}
			}
		}
	}()
	return c
}

func monitor(done chan bool) chan bool {
	out := make(chan bool)
	go func() {
		var last uint64
		start := time.Now()
		var elapsed time.Duration

	OUTER:
		for {
			select {
			case <-done:
				break OUTER
			case <-time.After(1 * time.Second):
				current := atomic.LoadUint64(&requestCount)
				fmt.Printf("%d combined requests per second (%d)\n", current-last, current)
				if current >= uint64(totalPings) { //|| current-last == 0
					break OUTER
				}
				last = current
			}
		}

		elapsed = time.Since(start)
		fmt.Printf("%f ns\n", float64(elapsed)/float64(requestCount))
		fmt.Printf("%d requests\n", requestCount)
		fmt.Printf("%f requests per second\n", float64(time.Second)/(float64(elapsed)/float64(requestCount)))
		fmt.Printf("elapsed: %s\r\n", elapsed)
		out <- true
		return
	}()
	return out
}
