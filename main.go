package main

import (
	"log"
	"os"
	"runtime"

	mulu "github.com/eliquious/mulu/server"
	// "github.com/pkg/profile"
)

func main() {
	runtime.GOMAXPROCS(8)
	// defer profile.Start(profile.MemProfile, profile.ProfilePath(".")).Stop()

	logger := log.New(os.Stdout, "logger: ", log.Lshortfile)
	server := mulu.NewServer(512*1024*1024, logger)
	server.Start(":9022")
}
