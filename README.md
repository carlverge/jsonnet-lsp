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

## Editor setup

### Emacs

[Emacs 29.1](https://www.masteringemacs.org/article/whats-new-in-emacs-29-1) has built in support for [eglot](https://github.com/joaotavora/eglot) and [use-package](https://www.gnu.org/software/emacs/manual/html_node/use-package/index.html). Assuming a functional package setup to download and install jsonnet-mode for Emacs. The below snippet will setup jsonnet-mode with jsonnet-lsp as language server using eglot.

    (use-package jsonnet-mode
      :ensure t
      :config
      (add-to-list 'eglot-server-programs
                   '(jsonnet-mode . ("jsonnet-lsp" "lsp")))
      :mode (
             ("\\.jsonnet\\'" . jsonnet-mode)
             ("\\.jsonnet.TEMPLATE\\'" . jsonnet-mode)
             )
      :hook
      (jsonnet-mode . (lambda()
                        (eglot-ensure))))

The above snippet assumes that `jsonnet-lsp` is installed into the PATH. Using the `runlsp.sh` script to dynamically build jsonnet-lsp on startup, a small update to the eglot-server-programs is needed.

      (add-to-list 'eglot-server-programs
                   '(jsonnet-mode . ("/full/path/to/repo/jsonnet-lsp/runlsp.sh")))

### Neovim
This recipe uses [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig) to setup jsonnet-lsp with neovim.
You will need to install the `nvim-lspconfig` plugin uses its instructions. Then, in you lua setup file (e.g. ~/.config/nvim/init.lua, see [the official docs](https://neovim.io/doc/user/lua-guide.html) for setup) you can add:
```lua
local nvim_lsp = require('lspconfig')
nvm_lsp.jsonnet_ls.setup{
    cmd = { "/path/to/jsonnet-lsp/runlsp.sh" },
    filetypes = { "jsonnet", "libsonnet", ".jsonnet.template" },
    root_dir = nvm_lsp.util.root_pattern(".git", vim.fn.getcwd()),
    settings = {},
}
```
