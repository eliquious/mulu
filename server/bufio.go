package server

import "io"

func NewFixedSizeWriter(w io.Writer, size int) *FixedSizeWriter {
	return &FixedSizeWriter{w, make([]byte, size), 0}
}

type FlushableWriter interface {
	Flush() error
	Write([]byte) (int, error)
}

type FixedSizeWriter struct {
	writer io.Writer
	buffer []byte
	offset int
}

func (f *FixedSizeWriter) Write(p []byte) (n int, err error) {
	if len(p) > len(f.buffer) || len(p) >= f.Available() {
		if f.offset > 0 {
			written, e := f.writer.Write(f.buffer[:f.offset])
			if e != nil {
				return written, e
			}
			n += written
		}

		// write new content
		written, e := f.writer.Write(p)
		n += written
		if e != nil {
			return n, e
		}

		f.Reset()
		return n, e
	}

	n = copy(f.buffer[f.offset:], p)
	f.offset += len(p)
	return
}

func (f *FixedSizeWriter) Flush() error {
	// write buffer content
	if f.offset > 0 {
		_, err := f.writer.Write(f.buffer[:f.offset])
		if err != nil {
			return err
		}
	}
	f.offset = 0
	return nil
}

func (f *FixedSizeWriter) Available() int {
	return len(f.buffer) - f.offset
}

func (f *FixedSizeWriter) Reset() {
	f.offset = 0
}

func (f *FixedSizeWriter) Buffered() int {
	return f.offset
}

func (f *FixedSizeWriter) Bytes() []byte {
	return f.buffer[:f.offset]
}
