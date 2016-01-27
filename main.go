package main

import (
	"runtime"

	// "github.com/pkg/profile"
	mulu "github.com/eliquious/mulu/server"

	"github.com/coocood/freecache"
)

func main() {
	runtime.GOMAXPROCS(8)
	// defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()

	cache := freecache.NewCache(5 * 1024 * 1024)
	cache.Set([]byte("key0"), []byte("value"), 0)
	cache.Set([]byte("key1"), []byte("value"), 0)
	cache.Set([]byte("key2"), []byte("value"), 0)
	cache.Set([]byte("key3"), []byte("value"), 0)
	cache.Set([]byte("key4"), []byte("value"), 0)
	cache.Set([]byte("key5"), []byte("value"), 0)
	cache.Set([]byte("key6"), []byte("value"), 0)
	cache.Set([]byte("key7"), []byte("value"), 0)
	cache.Set([]byte("key8"), []byte("value"), 0)

	server := mulu.Server{Cache: cache}
	server.Start(":9022")
}
