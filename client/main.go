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
)

var requestCount uint64
var totalPingsPerConnection uint64 = 1000000
var concurrentConnections uint64 = 64
var totalPings = concurrentConnections * totalPingsPerConnection

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
                if current >= uint64(totalPings) || current-last == 0 {
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

func (c *client) readLoop(wg *sync.WaitGroup) {
    defer wg.Done()

    rd := bufio.NewReader(c.conn)
    // buf := make([]byte, 1024)

    for atomic.LoadUint64(&c.revcd) < totalPingsPerConnection {

        line, _, err := rd.ReadLine()
        if err != nil {
            fmt.Println(err)
            return
        }

        if len(line) > 0 && line[0] == '-' {
            fmt.Println(string(line))
            return
        }

        atomic.AddUint64(&c.revcd, 1)
        atomic.AddUint64(&requestCount, 1)

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

func (c *client) writeLoop(wg *sync.WaitGroup) {
    defer wg.Done()

    wr := bufio.NewWriterSize(c.conn, 65536)
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
    }
    wr.Flush()
}

type client struct {
    sent  uint64
    revcd uint64
    conn  *net.TCPConn
}

func NewClient(wg *sync.WaitGroup) {
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
    go c.writeLoop(&w)
    go c.readLoop(&w)

    w.Wait()
    conn.Close()
}

func main() {
    runtime.GOMAXPROCS(2)

    var wg sync.WaitGroup
    done := make(chan bool)
    c := monitor(done)

    for i := uint64(0); i < concurrentConnections; i++ {
        wg.Add(1)
        go NewClient(&wg)
    }

    wg.Wait()
    done <- true
    <-c
}
