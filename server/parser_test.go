package server

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/coocood/freecache"
)

func TestParser(t *testing.T) {
}

func BenchmarkParserGet(b *testing.B) {
	cache := freecache.NewCache(0)
	logger := log.New(ioutil.Discard, "logger: ", log.Lshortfile)
	parser := Parser{logger: logger, writer: ioutil.Discard, cache: cache}
	line := []byte("GET key")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(line)
	}
}

func BenchmarkParserSet(b *testing.B) {
	cache := freecache.NewCache(0)
	logger := log.New(ioutil.Discard, "logger: ", log.Lshortfile)
	parser := Parser{logger: logger, writer: ioutil.Discard, cache: cache}
	line := []byte("SET key 0 value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(line)
	}
}
