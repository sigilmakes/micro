// Command micro-treesitter is the tree-sitter highlight helper for micro.
//
// It reads newline-framed requests from stdin and writes span replies to
// stdout. See ../../README.md for the wire format.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"micro-treesitter/internal/protocol"
	"micro-treesitter/internal/tsprovider"
)

var defaultGrammarDir = "./queries"

func main() {
	grammarDirFlag := flag.String("grammar-dir", "", "directory containing <grammar>/highlights.scm (default $MICRO_TS_GRAMMAR_DIR or "+defaultGrammarDir+")")
	verbose := flag.Bool("verbose", false, "log request errors to stderr")
	flag.Parse()

	grammarDir := *grammarDirFlag
	if grammarDir == "" {
		grammarDir = os.Getenv("MICRO_TS_GRAMMAR_DIR")
	}
	if grammarDir == "" {
		grammarDir = defaultGrammarDir
	}

	engine := tsprovider.New(grammarDir)
	defer engine.Close()

	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	writer := bufio.NewWriterSize(os.Stdout, 1<<20)

	if *verbose {
		log.Printf("micro-treesitter: grammar dir %s", grammarDir)
	}

	for {
		request, err := protocol.ReadRequest(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			var headerErr *protocol.HeaderError
			if errors.As(err, &headerErr) {
				_ = protocol.WriteError(writer, headerErr.ID, headerErr.Gen, headerErr.Message)
				if *verbose {
					log.Printf("micro-treesitter: %v", err)
				}
				// The unread payload length is unknown, so the stream cannot be
				// resynchronised. Exit after reporting the error.
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "micro-treesitter: %v\n", err)
			os.Exit(1)
		}

		spans, err := engine.Spans(request.ID, request.Filetype, request.Payload)
		if err != nil {
			if *verbose {
				log.Printf("micro-treesitter: request %d (%s): %v", request.ID, request.Filetype, err)
			}
			if err := protocol.WriteError(writer, request.ID, request.Gen, err.Error()); err != nil {
				fmt.Fprintf(os.Stderr, "micro-treesitter: write: %v\n", err)
				os.Exit(1)
			}
			continue
		}
		if err := protocol.WriteSpans(writer, request.ID, request.Gen, spans); err != nil {
			fmt.Fprintf(os.Stderr, "micro-treesitter: write: %v\n", err)
			os.Exit(1)
		}
	}
}
