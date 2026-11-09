// Package nix exposes the tree-sitter-nix grammar.
//
// The upstream module carries C sources but no Go bindings, so the parser and
// scanner are compiled here. See third_party/tree-sitter-nix/LICENSE.
package nix

/*
#cgo CFLAGS: -std=c11 -fPIC -I${SRCDIR}/../../../third_party/tree-sitter-nix/src
#include "../../../third_party/tree-sitter-nix/src/parser.c"
#include "../../../third_party/tree-sitter-nix/src/scanner.c"
*/
import "C"
import "unsafe"

// Language returns the tree-sitter-nix language.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_nix())
}
