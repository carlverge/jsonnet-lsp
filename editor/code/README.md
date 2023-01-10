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
