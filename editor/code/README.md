# jsonnet-lsp

This plugin aims to deliver a feature-rich, high quality, high performance IDE experience for Jsonnet. It is designed to work in large repositories with deeply nested imports.

## Features
* Syntax Highliting and Basic Editing features
* Snippets
* Custom linting code that is able to deal with large codebases
    * The analysis code is optimized for real-time linting, and can return in <5ms when the normal linter could take minutes.
* Formatting
* Delta text update support for efficient editing
* Designed to remain performant in large repos with many files open
* Automatic detection of `bazel-bin` for generated files
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

## Missing Features
These are features I consider pretty important that are still missing:
* More type deduction logic and analysis
* More IDE options (options for linting, jpath, etc)
* First class multi-dimension jsonnet support

## Development

* To develop the LSP, change the `jsonnet.lsp.binaryPath` setting to the `runlsp.sh` script in the root. Reloading the LSP in vscode (shift+cmd+p -> jsonnet: reload language server) will rebuild the server.
* To develop the client, open `editor/code` in vscode, and hit F5 to open a debug build of the client. Generally developing the LSP does not need a debug version of the client.

## Release

* Github actions are setup to publish to OpenVSX automatically when a release is created
* The LSP binaries are bundled into the `.vsix` extension, to help reduce bugs from version skew and simplify installation.
