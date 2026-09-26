package audit

import "io"

// bytesReader devolve um io.Reader sobre []byte sem importar bytes lá em cima.
func bytesReader(b []byte) io.Reader {
	return &sliceReader{buf: b}
}

type sliceReader struct {
	buf []byte
	pos int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	n := copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}
