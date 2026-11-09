package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"micro-treesitter/internal/tsprovider"
)

func TestReadRequest(t *testing.T) {
	payload := "package main\n\nfunc main() {}\n"
	frame := fmt.Sprintf("REQ\t42\t7\tgo\t%d\n%s", len(payload), payload)
	request, err := ReadRequest(bufio.NewReader(strings.NewReader(frame)))
	if err != nil {
		t.Fatal(err)
	}
	if request.ID != 42 || request.Gen != 7 || request.Filetype != "go" {
		t.Fatalf("header fields: %+v", request)
	}
	if string(request.Payload) != payload {
		t.Fatalf("payload: %q", request.Payload)
	}
	if _, err := ReadRequest(bufio.NewReader(strings.NewReader(""))); !errors.Is(err, io.EOF) {
		t.Fatalf("empty input: want io.EOF, got %v", err)
	}
}

func TestReadRequestMalformed(t *testing.T) {
	_, err := ReadRequest(bufio.NewReader(strings.NewReader("REQ\t3\t9\tgo\tx\n")))
	var headerErr *HeaderError
	if !errors.As(err, &headerErr) {
		t.Fatalf("want HeaderError, got %v", err)
	}
	if headerErr.ID != 3 || headerErr.Gen != 9 {
		t.Fatalf("best-effort ids: %+v", headerErr)
	}
}

func TestWriteSpans(t *testing.T) {
	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)
	spans := []tsprovider.Span{
		{Line: 0, Start: 0, End: 7, Group: "preproc"},
		{Line: 2, Start: 5, End: 9, Group: "constant.string"},
	}
	if err := WriteSpans(writer, 4, 11, spans); err != nil {
		t.Fatal(err)
	}
	want := "SPANS\t4\t11\t2\n0\t0\t7\tpreproc\n2\t5\t9\tconstant.string\n"
	if buffer.String() != want {
		t.Fatalf("got %q want %q", buffer.String(), want)
	}
}

func TestWriteErrorSanitisesMessage(t *testing.T) {
	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)
	if err := WriteError(writer, 1, 2, "bad\trequest\nsecond line"); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); got != "ERR\t1\t2\tbad request second line\n" {
		t.Fatalf("got %q", got)
	}
}
