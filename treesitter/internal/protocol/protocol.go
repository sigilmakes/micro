// Package protocol implements the micro-treesitter stdin/stdout framing.
package protocol

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"micro-treesitter/internal/tsprovider"
)

// MaxPayload bounds one request body. A larger declared length is rejected
// without reading it, because the stream cannot be resynchronised safely.
const MaxPayload = 64 << 20

// Request is one REQ frame: a header plus its exact payload.
type Request struct {
	ID       int64
	Gen      int64
	Filetype string
	Payload  []byte
}

// HeaderError reports a frame that could not be parsed. ID and Gen are the
// best-effort values from the header, so the caller can send an ERR frame.
type HeaderError struct {
	ID      int64
	Gen     int64
	Message string
}

func (e *HeaderError) Error() string { return e.Message }

// ReadRequest reads one request. It returns io.EOF only for a clean end of
// input before any header byte.
func ReadRequest(r *bufio.Reader) (*Request, error) {
	header, err := r.ReadString('\n')
	if err != nil {
		if err == io.EOF && header == "" {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("read header: %w", err)
	}
	header = strings.TrimSuffix(header, "\n")
	header = strings.TrimSuffix(header, "\r")
	fields := strings.Split(header, "\t")
	if len(fields) < 3 {
		return nil, &HeaderError{Message: "malformed header: want REQ\\t<id>\\t<gen>\\t<filetype>\\t<byte_len>"}
	}
	id, idErr := strconv.ParseInt(fields[1], 10, 64)
	gen, _ := strconv.ParseInt(fields[2], 10, 64)
	if idErr != nil {
		return nil, &HeaderError{Gen: gen, Message: "malformed header: invalid id"}
	}
	if len(fields) != 5 || fields[0] != "REQ" {
		return nil, &HeaderError{ID: id, Gen: gen, Message: "malformed header: want REQ\\t<id>\\t<gen>\\t<filetype>\\t<byte_len>"}
	}
	length, lengthErr := strconv.ParseInt(fields[4], 10, 64)
	if lengthErr != nil || length < 0 || length > MaxPayload {
		return nil, &HeaderError{ID: id, Gen: gen, Message: "malformed header: invalid byte_len"}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("read payload of %d bytes: %w", length, err)
	}
	return &Request{ID: id, Gen: gen, Filetype: fields[3], Payload: payload}, nil
}

// WriteSpans writes one SPANS frame and its span lines.
func WriteSpans(w *bufio.Writer, id, gen int64, spans []tsprovider.Span) error {
	if _, err := fmt.Fprintf(w, "SPANS\t%d\t%d\t%d\n", id, gen, len(spans)); err != nil {
		return err
	}
	for _, span := range spans {
		if _, err := fmt.Fprintf(w, "%d\t%d\t%d\t%s\n", span.Line, span.Start, span.End, span.Group); err != nil {
			return err
		}
	}
	return w.Flush()
}

// WriteError writes one ERR frame.
func WriteError(w *bufio.Writer, id, gen int64, message string) error {
	message = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(message)
	if _, err := fmt.Fprintf(w, "ERR\t%d\t%d\t%s\n", id, gen, message); err != nil {
		return err
	}
	return w.Flush()
}
