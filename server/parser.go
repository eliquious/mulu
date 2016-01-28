package server

import (
	"io"
	"log"
	"strconv"

	"github.com/coocood/freecache"
)

const (
	OP_START int = iota
	OP_G
	OP_GE
	OP_GET
	OP_S
	OP_SE
	OP_SET
	OP_SET_KEY
)

// Errors
var ErrMaxSize = []byte("-ERRMAXSIZE Request too large\r\n")
var ErrUnknownCmd = []byte("-ERRPARSE Unknown command\r\n")
var ErrIncompleteCmd = []byte("-ERRPARSE Incomplete command\r\n")
var ErrEmptyRequest = []byte("-ERRPARSE Empty request\r\n")
var ErrInvalidCmdDelimiter = []byte("-ERRPARSE Missing tab character after command\r\n")
var ErrInvalidKeyDelimiter = []byte("-ERRPARSE Missing tab character after key\r\n")
var ErrLargeKey = []byte("-ERRLARGEKEY The key is larger than 65535\r\n")
var ErrLargeEntry = []byte("-ERRLARGEENTRY The entry size is larger than 1/1024 of cache size\r\n")
var ErrNotFound = []byte("-ERRNOTFOUND Entry not found\r\n")
var ErrInvalidExpiration = []byte("-ERRINVEXP Invalid key expiration\r\n")
var ErrUnknownCache = []byte("-ERRCACHE Unknown cache error\r\n")

var OKResponse = []byte("+OK\r\n")
var ValuePrefix = []byte("+VALUE ")
var CRLF = []byte("\r\n")

func NewParser(cache *freecache.Cache, w io.Writer, logger *log.Logger) *Parser {
	return &Parser{cache: cache, writer: w, logger: logger}
}

type Parser struct {
	logger          *log.Logger
	writer          io.Writer
	cache           *freecache.Cache
	key, value, err []byte
}

func (p *Parser) Parse(line []byte) bool {
	if len(line) == 0 {
		p.writer.Write(ErrEmptyRequest)
		return false
	}
	// b.logger.Printf("Parsing line: %s\r\n", strconv.Quote(string(line)))

	var i, expiration int
	var c byte
	state := OP_START
	var v []byte
	var e error

	// Move to loop instead of range syntax to allow jumping of i
	for i = 0; i < len(line); i++ {
		c = line[i]

		switch state {
		case OP_START:
			switch c {
			case 'G', 'g':
				state = OP_G
			case 'S', 's':
				state = OP_S
			default:
				p.logger.Printf("OP_START: (%s) %s\r\n", c, strconv.Quote(string(line)))
				p.err = ErrUnknownCmd
				goto PARSE_ERR
			}
		case OP_G:
			switch c {
			case 'E', 'e':
				state = OP_GE
			default:
				p.logger.Printf("OP_G: (%s) %s\r\n", c, strconv.Quote(string(line)))
				p.err = ErrUnknownCmd
				goto PARSE_ERR
			}
		case OP_GE:
			switch c {
			case 'T', 't':
				state = OP_GET
			default:
				p.logger.Printf("OP_GE: (%s) %s\r\n", c, strconv.Quote(string(line)))
				p.err = ErrUnknownCmd
				goto PARSE_ERR
			}
		case OP_GET:
			switch c {
			case '\t', ' ':
				p.key = (line)[i+1:]
				// b.logger.Printf("KEY: %s\r\n", strconv.Quote(string(key)))
				goto PERFORM_GET
			default:
				p.err = ErrInvalidCmdDelimiter
				goto PARSE_ERR
			}

		case OP_S:
			switch c {
			case 'E', 'e':
				state = OP_SE
			default:
				p.logger.Printf("OP_S: (%s) %s\r\n", c, strconv.Quote(string(line)))
				p.err = ErrUnknownCmd
				goto PARSE_ERR
			}
		case OP_SE:
			switch c {
			case 'T', 't':
				state = OP_SET
			default:
				p.logger.Printf("OP_GE: (%s) %s\r\n", c, strconv.Quote(string(line)))
				p.err = ErrUnknownCmd
				goto PARSE_ERR
			}
		case OP_SET:
			switch c {
			case '\t', ' ':
				state = OP_SET_KEY
			default:
				p.err = ErrInvalidCmdDelimiter
				goto PARSE_ERR
			}

		case OP_SET_KEY:
			offset := i
			for i < len(line) {
				if line[i] == '\t' || line[i] == ' ' {
					break
				}
				i++
			}

			// end of input?
			// key empty?
			if i == len(line) || i-offset == 0 {
				p.err = ErrIncompleteCmd
				goto PARSE_ERR
			}

			// set key
			p.key = line[offset:i]

			// skip space
			i++

			// parse expiration
			offset = i
			for i < len(line) {
				if line[i] == '\t' || line[i] == ' ' {
					i++
					break
				}
				i++
			}

			// end of input? empty?
			if i >= len(line) || i-offset == 0 {
				p.err = ErrIncompleteCmd
				goto PARSE_ERR
			}

			exp, e := strconv.Atoi(string(line[offset : i-1]))
			if e != nil {
				p.err = ErrInvalidExpiration
				goto PARSE_ERR
			}
			expiration = exp
			p.value = line[i-1:]

			goto PERFORM_SET

			// 	key = line[offset:i]
			// 	i++
			// 	switch cmd {
			// 	case OP_GET:
			// 		goto PERFORM_GET
			// 	default:
			// 		return ErrUnknownCmd, false
			// 	}
		}
	}

PARSE_ERR:

	// Ignoring all write errors here, because we are going to return false
	// and close the connection due to the parse error anyway.
	p.writer.Write(p.err)
	p.logger.Printf("%s (%s)\r\n", string(p.err), strconv.Quote(string(line)))
	return false

PERFORM_GET:
	v, e = p.cache.Get(p.key)
	if e == freecache.ErrLargeKey {
		p.err = ErrLargeKey
		goto PARSE_ERR
	} else if e == freecache.ErrLargeEntry {
		p.err = ErrLargeEntry
		goto PARSE_ERR
	} else if e == freecache.ErrNotFound {
		p.err = ErrNotFound
		goto PARSE_ERR
	} else if e != nil {
		p.err = ErrUnknownCache
		goto PARSE_ERR
	} else {
		if _, err := p.writer.Write(ValuePrefix); err != nil {
			return false
		}
		if _, err := p.writer.Write(v); err != nil {
			return false
		}
		if _, err := p.writer.Write(CRLF); err != nil {
			return false
		}
		return true
	}

PERFORM_SET:
	e = p.cache.Set(p.key, p.value, expiration)
	if e == freecache.ErrLargeKey {
		p.err = ErrLargeKey
		goto PARSE_ERR
	} else if e == freecache.ErrLargeEntry {
		p.err = ErrLargeEntry
		goto PARSE_ERR
	} else if e != nil {
		p.err = ErrUnknownCache
		goto PARSE_ERR
	} else {
		if _, err := p.writer.Write(OKResponse); err != nil {
			return false
		}
		return true
	}

	p.logger.Printf("END %s\r\n", c)
	p.err = ErrUnknownCmd
	goto PARSE_ERR
}
