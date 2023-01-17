#!/bin/bash
# binaryPath for LSP development. Automatically rebuilds on reload.
(cd $(dirname $0) && go run main.go lsp)
