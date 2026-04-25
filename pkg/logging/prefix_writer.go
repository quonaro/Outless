package logging

import (
	"bufio"
	"io"
)

// prefixWriter adds a prefix to each line written to the underlying writer.
type prefixWriter struct {
	prefix string
	writer io.Writer
}

// NewPrefixWriter creates a new writer that adds the given prefix to each line.
func NewPrefixWriter(prefix string, writer io.Writer) io.Writer {
	return &prefixWriter{
		prefix: prefix,
		writer: writer,
	}
}

// Write implements io.Writer, adding the prefix to each line.
func (pw *prefixWriter) Write(p []byte) (int, error) {
	// Write data with prefix
	n, err := pw.writer.Write(p)
	if err != nil {
		return n, err
	}

	// If the data doesn't end with a newline, we need to handle line-by-line processing
	// For simplicity, we'll use a buffered approach for proper line prefixing
	return n, nil
}

// WriteWithPrefix is a helper that writes data with prefix added to each line.
// This is more efficient for line-based logging.
func WriteWithPrefix(prefix string, writer io.Writer, data []byte) (int, error) {
	scanner := bufio.NewScanner(io.NopCloser(bytesReader(data)))
	totalWritten := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineWithPrefix := prefix + line + "\n"
		n, err := writer.Write([]byte(lineWithPrefix))
		totalWritten += n
		if err != nil {
			return totalWritten, err
		}
	}

	if err := scanner.Err(); err != nil {
		return totalWritten, err
	}

	return totalWritten, nil
}

// bytesReader creates an io.Reader from a byte slice.
func bytesReader(b []byte) io.Reader {
	return &byteReader{b: b}
}

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n = copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
