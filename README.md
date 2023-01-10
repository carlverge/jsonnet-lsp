# jsonnet-lsp
Jsonnet Language Server

## Features
* Syntax Highliting and Basic Editing features
* Snippets
* Linting and Syntax Errors
* Formatting
* Delta text update support for efficient editing
* Designed to remain performant in large repos with many files open
* Type and Value Deduction
    * Supports imported files
    * Able to follow variables, function return values, and array/object indexing
* Autocomplete
    * Stdlib support with documentation and typed signatures
    * Scoped variable completion
    * Dotted autocomplete
    * Template object field completion
    * Import path completion for files
* Go to Definition
    * Can follow definitions in other files, including json files
* Hover Information
* Function Signature Help
* AST Recovery
    * The LSP is able recover common syntax issues while typing (like a missing semicolon) for a smoother experience

## Development

* To develop the LSP, change the `jsonnet.lsp.binaryPath` setting to the `runlsp.sh` script in the root. Reloading the LSP in vscode (shift+cmd+p -> jsonnet: reload language server) will rebuild the server.
* To develop the client, open `editor/code` in vscode, and hit F5 to open a debug build of the client. Generally developing the LSP does not need a debug version of the client.

## Release

* Github actions are setup to publish to OpenVSX automatically when a release is created
* The LSP binaries are bundled into the `.vsix` extension, to help reduce bugs from version skew and simplify installation.