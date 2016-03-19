package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/coocood/freecache"
	mulu "github.com/eliquious/mulu/server"
	// "github.com/pkg/profile"
)

func main() {
	runtime.GOMAXPROCS(8)
	// defer profile.Start(profile.MemProfile, profile.ProfilePath(".")).Stop()

	logger := log.New(os.Stdout, "logger: ", log.Lshortfile)
	cache := freecache.NewCache(512 * 1024 * 1024)
	for index := 0; index < 128; index++ {
		cache.Set([]byte(fmt.Sprintf("key%d", index)), []byte("value"), 0)
	}
	server := mulu.NewServer(cache, logger)
	server.Start(":9022")
}
