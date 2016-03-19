package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcrowley/go-metrics"
)

var requestCount uint64
var totalPingsPerConnection uint64 = 100000
var concurrentConnections uint64 = 128
var totalPings = concurrentConnections * totalPingsPerConnection

func main() {
	runtime.GOMAXPROCS(8)

	timer := metrics.NewTimer()
	done := make(chan bool, 1)
	exit := monitor(done, timer)

	// start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < int(concurrentConnections); i++ {
		wg.Add(1)
		go client(&wg, timer)
	}

	wg.Wait()
	// duration := time.Now().Sub(start)
	<-exit
	// fmt.Println(duration)
	// fmt.Println(uint64(duration) / concurrentConnections / totalPingsPerConnection)
	// fmt.Println(float64(concurrentConnections*totalPingsPerConnection) / duration.Seconds())
}

func client(wg *sync.WaitGroup, timer metrics.Timer) {
	defer wg.Done()
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

	for i := 0; i < int(totalPingsPerConnection); i++ {
		outBuf := []byte(fmt.Sprintf("GET key%d\r\n", i&127))
		start := time.Now()
		_, err = conn.Write(outBuf)
		if err != nil {
			println("Write to server failed:", err.Error())
			os.Exit(1)
		}

		// println("write to server = ", strEcho)

		reply := make([]byte, 1024)
		_, err = conn.Read(reply)
		if err != nil {
			println("Write to server failed:", err.Error())
			os.Exit(1)
		}

		atomic.AddUint64(&requestCount, 1)
		timer.UpdateSince(start)
		// hist.Update(time.Now().Sub(start).Nanoseconds())
	}

	// println("reply from server=", string(reply))

	conn.Close()
}

func monitor(done chan bool, timer metrics.Timer) chan bool {
	out := make(chan bool)
	go func() {
		start := time.Now()
		var elapsed time.Duration
		rps := metrics.NewHistogram(metrics.NewUniformSample(10000))

	OUTER:
		for {
			select {
			case <-done:
				break OUTER
			case <-time.After(1 * time.Second):
				current := atomic.LoadUint64(&requestCount)
				rate := timer.RateMean()
				rps.Update(int64(rate))
				fmt.Printf("rps: %f\t mean lat: %s\t total: %d\n", rate, time.Duration(timer.Mean()), timer.Count())
				if current >= uint64(totalPings) {
					break OUTER
				}
			}
		}

		elapsed = time.Since(start)
		fmt.Printf("\nTotal requests: %d\n", timer.Count())
		fmt.Printf("Concurrent Connections: %d\n", concurrentConnections)
		fmt.Printf("Requests per Second: %f\n", rps.Mean())
		fmt.Printf("Mean Latency: %s\n", time.Duration(timer.Mean()))
		fmt.Printf("Elapsed: %s\r\n", elapsed)
		fmt.Printf("\nLatency Histogram:\n")

		percs := []float64{0.000, 0.250, 0.500, 0.750, 0.900, 0.950, 0.950, 0.960, 0.970, 0.980, 0.990, 0.991, 0.992, 0.993, 0.994, 0.995, 0.996, 0.997, 0.998, 0.999, 1.000}

		values := timer.Percentiles(percs)
		for i, val := range values {
			fmt.Printf("% 3.2f %s\n", (percs[i])*100.0, time.Duration(int64(val)))
		}
		// for p := 0.0; p < 1; p += 0.05 {
		// 	fmt.Printf("%02.3f % -9.2f\n", p, time.Duration(int64(timer.Percentile(p))))
		// }
		// for p := 0.95; p < 0.99; p += 0.01 {
		// 	fmt.Printf("%02.3f % -9.2f\n", p, time.Duration(int64(timer.Percentile(p))))
		// }
		// for p := 0.99; p <= 1.0; p += 0.001 {
		// 	fmt.Printf("%02.3f % -9.2f\n", p, time.Duration(int64(timer.Percentile(p))))
		// }
		out <- true
		return
	}()
	return out
}
