package main

import (
	"context"
	"fmt"
	"os"

	"github.com/carlverge/jsonnet-lsp/pkg/lsp"
)

func doLSP() error {
	// swap out process-level stdout right away to ensure that nothing else writes to it
	// otherwise it will desync the jsonrpc stream
	oldout := os.Stdout
	os.Stdout = os.Stderr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return lsp.RunServer(ctx, oldout)
}

func main() {
	if err := doLSP(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
