package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/carlverge/jsonnet-lsp/pkg/lsp"
)

type cmd struct {
	Fn   func(args []string) error
	Help string
}

var subcommands = map[string]cmd{
	"lsp": {Fn: doLSP, Help: "Run the jsonnet language server. Uses stdin/stdout for communication."},
}

func fmtUsage(cmds map[string]cmd) string {
	names := []string{}
	for n := range cmds {
		names = append(names, n)
	}
	sort.Strings(names)
	res := strings.Builder{}
	res.WriteString("usage:\n")
	res.WriteString("    help - show this help message\n")
	for _, n := range names {
		res.WriteString(fmt.Sprintf("    %s - %s\n", n, cmds[n].Help))
	}
	return res.String()
}

func dispatch(args []string, cmds map[string]cmd) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		os.Stdout.WriteString(fmtUsage(cmds))
		return nil
	}

	sub, ok := cmds[args[0]]
	if !ok {
		return fmt.Errorf("unknown subcommand %s", args[0])
	}
	return sub.Fn(args[1:])
}

func doLSP(args []string) error {
	// swap out process-level stdout right away to ensure that nothing else writes to it
	// otherwise it will desync the jsonrpc stream
	oldout := os.Stdout
	os.Stdout = os.Stderr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return lsp.RunServer(ctx, oldout)
}

func main() {
	if err := dispatch(os.Args[1:], subcommands); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
