package writer

type ByteWriter struct {
	data []byte
}

func New() *ByteWriter {
	return &ByteWriter{
		data: make([]byte, 0),
	}
}

func (b *ByteWriter) Write(data []byte) (int, error) {
	b.data = append(b.data, data...)
	return len(data), nil
}

func (b *ByteWriter) String() string {
	return string(b.data)
}
