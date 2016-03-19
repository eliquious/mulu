//CLIENT
package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eliquious/mulu/server"
	"github.com/rcrowley/go-metrics"
)

var requestCount uint64
var totalPingsPerConnection uint64 = 100000000
var concurrentConnections uint64 = 1
var totalPings = concurrentConnections * totalPingsPerConnection

func monitor(done chan bool, write metrics.Meter, read metrics.Meter) chan bool {
	out := make(chan bool)
	go func() {
		start := time.Now()
		var elapsed time.Duration
		wrps := metrics.NewHistogram(metrics.NewUniformSample(10000))
		rrps := metrics.NewHistogram(metrics.NewUniformSample(10000))

	OUTER:
		for {
			select {
			case <-done:
				break OUTER
			case <-time.After(1 * time.Second):
				current := atomic.LoadUint64(&requestCount)
				wRate := write.RateMean()
				wrps.Update(int64(wRate))
				rRate := read.RateMean()
				rrps.Update(int64(rRate))

				fmt.Printf("write rps: %f (%d)\t read rps: %f\t (%d)\n", wRate, write.Count(), rRate, read.Count())
				if current >= uint64(totalPings) {
					break OUTER
				}
			}
		}

		elapsed = time.Since(start)
		fmt.Printf("\nTotal requests: %d\n", totalPings)
		fmt.Printf("Concurrent Connections: %d\n", concurrentConnections)
		fmt.Printf("Average Write Requests per Second: %f\n", wrps.Mean())
		fmt.Printf("Average Read Requests per Second: %f\n", rrps.Mean())
		fmt.Printf("Elapsed: %s\r\n", elapsed)
		// fmt.Printf("\nLatency Histogram:\n")

		// percs := []float64{0.000, 0.250, 0.500, 0.750, 0.900, 0.950, 0.950, 0.960, 0.970, 0.980, 0.990, 0.991, 0.992, 0.993, 0.994, 0.995, 0.996, 0.997, 0.998, 0.999, 1.000}

		// values := timer.Percentiles(percs)
		// for i, val := range values {
		// 	fmt.Printf("% 3.2f %s\n", (percs[i])*100.0, time.Duration(int64(val)))
		// }
		out <- true
		return
	}()
	return out
}

// func monitor(done chan bool) chan bool {
//     out := make(chan bool)
//     go func() {
//         var last uint64
//         start := time.Now()
//         var elapsed time.Duration
//
//     OUTER:
//         for {
//             select {
//             case <-done:
//                 break OUTER
//             case <-time.After(1 * time.Second):
//                 current := atomic.LoadUint64(&requestCount)
//                 fmt.Printf("%d combined requests per second (%d)\n", current-last, current)
//                 if current >= uint64(totalPings) || current-last == 0 {
//                     break OUTER
//                 }
//                 last = current
//             }
//         }
//
//         elapsed = time.Since(start)
//         fmt.Printf("%f ns\n", float64(elapsed)/float64(requestCount))
//         fmt.Printf("%d requests\n", requestCount)
//         fmt.Printf("%f requests per second\n", float64(time.Second)/(float64(elapsed)/float64(requestCount)))
//         fmt.Printf("elapsed: %s\r\n", elapsed)
//         out <- true
//         return
//     }()
//     return out
// }

func (c *client) readLoop(wg *sync.WaitGroup, meter metrics.Meter) {
	defer wg.Done()

	rd := bufio.NewReaderSize(c.conn, 1024*1024)
	scanner := bufio.NewScanner(rd)
	// buf := make([]byte, bufio.MaxScanTokenSize)
	// scanner.Buffer(buf, bufio.MaxScanTokenSize)

	for atomic.LoadUint64(&c.revcd) < totalPingsPerConnection {

		if !scanner.Scan() {
			fmt.Println(scanner.Err())
			return
		}
		// line, _, err := rd.ReadLine()
		// if err != nil {
		// 	fmt.Println(err)
		// 	return
		// }
		// line := scanner.Text()
		// if len(line) > 0 && line[0] == '-' {
		// 	fmt.Println(string(line))
		// 	return
		// }

		atomic.AddUint64(&c.revcd, 1)
		atomic.AddUint64(&requestCount, 1)
		meter.Mark(1)

		// n, err := rd.Read(buf)
		// if n > 0 {
		//     // fmt.Println(string(buf[:n]))
		//     atomic.AddUint64(&c.revcd, 1)
		//     atomic.AddUint64(&requestCount, 1)
		// } else if err == io.EOF {
		//     return
		// }
		// if err != nil && err != io.EOF {
		//     fmt.Println(err)
		//     return
		// }
	}
}

func (c *client) writeLoop(wg *sync.WaitGroup, meter metrics.Meter) {
	defer wg.Done()

	// wr := bufio.NewWriterSize(c.conn, 65536)
	wr := server.NewFixedSizeWriter(c.conn, 1024*1024)
	outBuf := []byte(fmt.Sprintf("GET key%d\r\n", rand.Intn(8)))
	// outBuf := []byte(fmt.Sprintf("SET key%d 0 value\r\n", rand.Intn(int(concurrentConnections)*8)))

	for atomic.LoadUint64(&c.sent) < totalPingsPerConnection {

		n, err := wr.Write(outBuf)
		if n > 0 {
			// wr.Flush()
		}
		if err != nil && err != io.EOF {
			fmt.Println(err)
			return
		}
		atomic.AddUint64(&c.sent, 1)
		meter.Mark(1)
	}
	wr.Flush()
}

type client struct {
	sent  uint64
	revcd uint64
	conn  *net.TCPConn
}

func NewClient(wg *sync.WaitGroup, write, read metrics.Meter) {
	defer wg.Done()

	tcpAddr, _ := net.ResolveTCPAddr("tcp4", "localhost:9022")
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println(err)
		return
	}

	var w sync.WaitGroup
	c := client{conn: conn}

	w.Add(2)
	go c.writeLoop(&w, write)
	go c.readLoop(&w, read)

	w.Wait()
	conn.Close()
}

func main() {
	runtime.GOMAXPROCS(2)

	var wg sync.WaitGroup
	writeMeter := metrics.NewMeter()
	readMeter := metrics.NewMeter()
	done := make(chan bool, 1)
	c := monitor(done, writeMeter, readMeter)

	for i := uint64(0); i < concurrentConnections; i++ {
		wg.Add(1)
		go NewClient(&wg, writeMeter, readMeter)
	}

	wg.Wait()
	done <- true
	<-c
}
