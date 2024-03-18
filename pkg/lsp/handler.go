package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlverge/jsonnet-lsp/pkg/analysis"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/formatter"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)


type DiagConfiguration struct {
	Linter   bool `json:"linter"`
	Evaluate bool `json:"evaluate"`
}

type FmtConfiguration struct {
	Indent           int    `json:"indent"`
	MaxBlankLines    int    `json:"maxBlankLines"`
	StringStyle      string `json:"stringStyle"`
	CommentStyle     string `json:"commentStyle"`
	PrettyFieldNames bool   `json:"prettyFieldNames"`
	PadArrays        bool   `json:"padArrays"`
	PadObjects       bool   `json:"padObjects"`
	SortImports      bool   `json:"sortImports"`
	ImplicitPlus     bool   `json:"implicitPlus"`
}

func defaultConfiguration() *Configuration {
	return &Configuration{
		Diag: DiagConfiguration{
			Linter:   true,
			Evaluate: false,
		},
		Fmt: FmtConfiguration{
			Indent:           2,
			StringStyle:      "\"",
			CommentStyle:     "//",
			MaxBlankLines:    2,
			PrettyFieldNames: true,
			PadArrays:        false,
			PadObjects:       true,
			ImplicitPlus:     true,
			SortImports:      true,
		},
	}
}

type Configuration struct {
	Diag   DiagConfiguration `json:"diag"`
	JPaths []string          `json:"jpaths"`
	Fmt    FmtConfiguration  `json:"fmt"`
}

func (c *Configuration) FormatterOptions() formatter.Options {
	if c == nil {
		return formatter.DefaultOptions()
	}

	opts := formatter.Options{
		Indent:           c.Fmt.Indent,
		MaxBlankLines:    c.Fmt.MaxBlankLines,
		PrettyFieldNames: c.Fmt.PrettyFieldNames,
		PadArrays:        c.Fmt.PadArrays,
		PadObjects:       c.Fmt.PadObjects,
		SortImports:      c.Fmt.SortImports,
		UseImplicitPlus:  c.Fmt.ImplicitPlus,
	}
	switch c.Fmt.StringStyle {
	case "\"":
		opts.StringStyle = formatter.StringStyleDouble
	case "'":
		opts.StringStyle = formatter.StringStyleSingle
	default:
		opts.StringStyle = formatter.StringStyleLeave
	}

	switch c.Fmt.CommentStyle {
	case "#":
		opts.CommentStyle = formatter.CommentStyleHash
	case "//":
		opts.CommentStyle = formatter.CommentStyleSlash
	default:
		opts.CommentStyle = formatter.CommentStyleLeave
	}

	return opts
}

func (s *Server) Handler() jsonrpc2.Handler {
	serverHandler := protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler)
	return serverHandler
}

func (s *Server) Shutdown(ctx context.Context) (err error) {
	return nil
}

func (s *Server) Exit(ctx context.Context) (err error) {
	s.cancel()
	return nil
}

func (s *Server) Initialized(ctx context.Context, params *protocol.InitializedParams) (err error) {
	return nil
}

func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (result *protocol.InitializeResult, err error) {

	s.rootURI = findRootDirectory(params)
	// s.rootFS = os.DirFS("/")
	s.rootFS = os.DirFS(s.rootURI.Filename())

	// Check for bazel generated output directory
	if _, err := fs.Stat(s.rootFS, "bazel-bin"); err == nil {
		s.searchPaths = append(s.searchPaths, "bazel-bin")
	} else {
		logf("no bazel-bin dir: %v", err)
	}

	s.importer = &OverlayImporter{overlay: s.overlay, rootURI: s.rootURI, rootFS: s.rootFS, paths: s.searchPaths}

	_ = s.notifier.LogMessage(ctx, &protocol.LogMessageParams{
		Message: "Jsonnet LSP Server Initialized",
		Type:    protocol.MessageTypeLog,
	})

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				Change:    protocol.TextDocumentSyncKindIncremental,
				OpenClose: true,
				Save:      &protocol.SaveOptions{},
			},
			SignatureHelpProvider: &protocol.SignatureHelpOptions{
				TriggerCharacters:   []string{"("},
				RetriggerCharacters: []string{","},
			},
			DocumentSymbolProvider: true,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{".", "/"},
			},
			DocumentFormattingProvider: true,
			HoverProvider:              true,
			DefinitionProvider:         true,
		},
	}, nil
}

func (s *Server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) (err error) {
	data, _ := json.Marshal(params.Settings)
	logf("did change config: %s", string(data))
	newcfg := &Configuration{}
	if err := json.Unmarshal(data, newcfg); err != nil {
		logf("failed to parse new configuration: %+v", err)
		return nil
	}

	// TODO(@carlverge): Rethink how paths are threaded through the code, this is getting too messy.
	s.importer.SetJPaths(newcfg.JPaths)

	// Racy in the sense we could see an old pointer, but that is OK.
	s.config = newcfg

	return nil
}

func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	logf("did-open: uri=%s ver=%d txtlen=%d", params.TextDocument.URI, params.TextDocument.Version, len(params.TextDocument.Text))
	s.overlay.Replace(
		params.TextDocument.URI,
		int64(params.TextDocument.Version),
		params.TextDocument.Text,
		parseJsonnetFn(params.TextDocument.URI),
		s.processFileUpdateFn(ctx, params.TextDocument.URI),
	)
	return nil
}

func lastCharIsDot(ce []protocol.TextDocumentContentChangeEvent) bool {
	if len(ce) == 0 {
		return false
	}
	text := ce[len(ce)-1].Text
	return len(text) > 0 && text[len(text)-1] == '.'
}

func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	tracef("did-change: uri=%s ver=%d changes=%d", params.TextDocument.URI, params.TextDocument.Version, len(params.ContentChanges))
	s.overlay.Update(
		params.TextDocument.URI,
		int64(params.TextDocument.Version),
		convChangeEvents(params.ContentChanges),
		parseJsonnetFn(params.TextDocument.URI),
		s.processFileUpdateFn(ctx, params.TextDocument.URI),
	)
	s.lastCharIsDot = lastCharIsDot(params.ContentChanges)
	return nil
}

func (s *Server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) (err error) {
	tracef("did-save: uri=%s", params.TextDocument.URI)
	return nil
}

func (s *Server) DidClose(_ context.Context, params *protocol.DidCloseTextDocumentParams) (err error) {
	logf("did-close: uri=%s", params.TextDocument.URI)
	s.overlay.Close(params.TextDocument.URI)
	return nil
}

// isObjectFieldsCompletion checks for the situation where there is an object being filled out
// with a template object (which is `objVar + {}`  or `objVar{}` in code, typically). Instead of
// showing local variables, show remaining fields that can be completed.
func isObjectFieldsCompletion(stk []ast.Node, resolver analysis.Resolver) []analysis.Field {
	if len(stk) < 2 {
		return nil
	}
	// this maps to usage like: template{f1: false, f2: 1234}
	bin, _ := stk[len(stk)-2].(*ast.Binary)
	obj, _ := stk[len(stk)-1].(*ast.DesugaredObject)
	if bin == nil || obj == nil || bin.Op != ast.BopPlus {
		return nil
	}
	lhs := analysis.NodeToValue(bin.Left, resolver)

	seenFields := map[string]bool{}
	for _, fld := range obj.Fields {
		if fn, ok := fld.Name.(*ast.LiteralString); ok {
			seenFields[fn.Value] = true
		}
	}

	// if the lhs (template in the above example) is an object with fields
	if lhs != nil && lhs.Object != nil && len(lhs.Object.Fields) > 0 {
		res := []analysis.Field{}
		// If the user has already filled out a field in the template, do not show it in the
		// completion list (or if the field is hidden)
		for _, fld := range lhs.Object.Fields {
			if seenFields[fld.Name] || fld.Hidden {
				continue
			}
			res = append(res, fld)
		}
		return res
	}

	return nil
}

var typeToCompletionKindMap = map[analysis.ValueType]protocol.CompletionItemKind{
	analysis.FunctionType: protocol.CompletionItemKindFunction,
	analysis.ObjectType:   protocol.CompletionItemKindStruct,
	analysis.ArrayType:    protocol.CompletionItemKindVariable,
	analysis.BooleanType:  protocol.CompletionItemKindVariable,
	analysis.AnyType:      protocol.CompletionItemKindVariable,
	analysis.NullType:     protocol.CompletionItemKindVariable,
	analysis.NumberType:   protocol.CompletionItemKindVariable,
	analysis.StringType:   protocol.CompletionItemKindVariable,
}

func typeToCompletionKind(tp analysis.ValueType, dflt protocol.CompletionItemKind) protocol.CompletionItemKind {
	v, ok := typeToCompletionKindMap[tp]
	if ok {
		return v
	}
	return dflt
}

func valueToDetail(v *analysis.Value) string {
	if v.Function != nil {
		return "function" + v.Function.String()
	}
	if v.Type == analysis.StringType && len(v.Comment) == 1 {
		return fmt.Sprintf("string(%q)", v.Comment[0])
	}
	if v.Type == analysis.NumberType && len(v.Comment) == 1 {
		return fmt.Sprintf("number(%s)", v.Comment[0])
	}
	return v.Type.String()
}

// precompute these as they are numerous and commonly used
// this also lets us bypass the issue of their not having a real
// ast node associated with them
var stdlibCompletions = func() (res []protocol.CompletionItem) {
	for name, val := range analysis.StdLibFunctions {

		res = append(res, protocol.CompletionItem{
			Label:         name,
			Detail:        name + val.String(),
			Documentation: &protocol.MarkupContent{Kind: protocol.Markdown, Value: strings.Join(val.Comment, "\n")},
			Kind:          protocol.CompletionItemKindFunction,
		})
	}
	return res
}()

func (s *Server) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	res := &protocol.CompletionList{IsIncomplete: false, Items: []protocol.CompletionItem{}}
	resolver := s.NewResolver(params.TextDocument.URI)
	if resolver == nil {
		return res, nil
	}

	isDotComplete := s.lastCharIsDot || (params.Context != nil && params.Context.TriggerCharacter == ".")
	isSlashComplete := params.Context != nil && params.Context.TriggerCharacter == "/"

	pos := protoToPos(params.Position)
	if isDotComplete {
		pos.Column--
	}
	node, stack := resolver.NodeAt(pos)

	// Import file completion
	if imp, ok := node.(*ast.Import); ok {
		// always search a directory
		path := filepath.Dir(imp.File.Value)
		if finfo, err := fs.Stat(s.rootFS, filepath.Clean(imp.File.Value)); err == nil && finfo.IsDir() {
			path = filepath.Clean(imp.File.Value)
		}

		seen := map[string]bool{}
		ents := []fs.DirEntry{}

		// Dedup files/directories from search paths
		for _, sp := range append(append([]string{""}, s.searchPaths...), s.config.JPaths...) {
			entries, _ := fs.ReadDir(s.rootFS, filepath.Join(sp, path))
			for _, ent := range entries {
				if seen[ent.Name()] {
					continue
				}
				ents = append(ents, ent)
				seen[ent.Name()] = true
			}
		}

		for _, m := range ents {
			if strings.HasPrefix(m.Name(), ".") {
				continue
			}
			kind := protocol.CompletionItemKindFile
			if m.IsDir() {
				kind = protocol.CompletionItemKindFolder
			}

			res.Items = append(res.Items, protocol.CompletionItem{
				Label: m.Name(),
				Kind:  kind,
			})
		}
		return res, nil
	}

	// This is only for file completion. If we didn't match an import node
	// above, then return without trying to complete anything.
	if isSlashComplete {
		return res, nil
	}

	if isDotComplete {
		topVal := analysis.NodeToValue(node, resolver)
		if topVal.Object == nil {
			return res, nil
		}

		if topVal == analysis.StdLibValue {
			res.Items = stdlibCompletions
			return res, nil
		}

		for _, fld := range topVal.Object.Fields {
			fldVal := analysis.NodeToValue(fld.Node, resolver)

			res.Items = append(res.Items, protocol.CompletionItem{
				Label:         fld.Name,
				InsertText:    analysis.SafeIdent(fld.Name),
				Detail:        valueToDetail(fldVal),
				Documentation: strings.Join(fld.Comment, "\n"),
				Kind:          typeToCompletionKind(fld.Type, protocol.CompletionItemKindField),
			})
		}
		return res, nil
	}

	if flds := isObjectFieldsCompletion(stack, resolver); flds != nil {
		for _, fld := range flds {
			res.Items = append(res.Items, protocol.CompletionItem{
				Label:            fld.Name,
				InsertText:       analysis.SafeIdent(fld.Name) + ": $1,$0",
				InsertTextFormat: protocol.InsertTextFormatSnippet,
				Detail:           fld.Type.String(),
				Documentation:    strings.Join(fld.Comment, "\n"),
				Kind:             protocol.CompletionItemKindField,
			})
		}
		return res, nil
	}

	for name, v := range resolver.Vars(node) {
		if v.Node != nil {
			val := analysis.NodeToValue(v.Node, resolver)

			res.Items = append(res.Items, protocol.CompletionItem{
				Label:         name,
				InsertText:    name,
				Detail:        val.Type.String(),
				Documentation: strings.Join(val.Comment, "\n"),
				Kind:          typeToCompletionKind(val.Type, protocol.CompletionItemKindVariable),
				SortText:      fmt.Sprintf("%3d_%s", v.StackPos, name),
			})
		} else {
			res.Items = append(res.Items, protocol.CompletionItem{
				Label:    name,
				Kind:     protocol.CompletionItemKindVariable,
				SortText: fmt.Sprintf("%3d_%s", 0, name),
			})
		}
	}

	return res, nil
}

func (s *Server) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	res := []interface{}{}
	root := s.getCurrentAST(params.TextDocument.URI)
	if root == nil {
		return res, nil
	}

	locals, _ := analysis.UnwindLocals(root)
	for _, name := range locals.Names() {
		v := locals.Get(name)
		res = append(res, protocol.DocumentSymbol{
			Name:           string(name),
			Kind:           protocol.SymbolKindVariable,
			Detail:         v.Type.String(),
			Range:          rangeToProto(v.Loc),
			SelectionRange: rangeToProto(v.Loc),
		})
	}

	return res, nil
}

func (s *Server) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	resolver := s.NewResolver(params.TextDocument.URI)
	if resolver == nil {
		return &protocol.SignatureHelp{Signatures: []protocol.SignatureInformation{}}, nil
	}

	node, _ := resolver.NodeAt(protoToPos(params.Position))
	if node == nil {
		return &protocol.SignatureHelp{Signatures: []protocol.SignatureInformation{}}, nil
	}

	apply, ok := node.(*ast.Apply)
	if !ok {
		return &protocol.SignatureHelp{Signatures: []protocol.SignatureInformation{}}, nil
	}

	targ := analysis.NodeToValue(apply.Target, resolver)
	if targ.Function == nil {
		return &protocol.SignatureHelp{Signatures: []protocol.SignatureInformation{}}, nil
	}

	// for each positional param, the active is at least len(positional)
	// if there are named, find the first named parameter in order that is
	// not in the named params list
	// The AST doesn't parse with partial named params, so we can't fully
	// properly highlight the active named (without gnarly string parsing)

	activeParam := 0
	if len(apply.Arguments.Positional) < len(targ.Function.Params) {
		seenNamed := map[string]bool{}
		for i := range apply.Arguments.Positional {
			seenNamed[targ.Function.Params[i].Name] = true
		}
		for _, p := range apply.Arguments.Named {
			seenNamed[string(p.Name)] = true
		}
		for i, p := range targ.Function.Params {
			if seenNamed[p.Name] {
				continue
			}
			activeParam = i
			break
		}
	}

	fnName := "function"
	switch name := apply.Target.(type) {
	case *ast.Index:
		if sl, ok := name.Index.(*ast.LiteralString); ok {
			fnName = sl.Value
		}
	case *ast.Var:
		fnName = string(name.Id)
	}

	sigp := []protocol.ParameterInformation{}
	for _, param := range targ.Function.Params {
		sigp = append(sigp, protocol.ParameterInformation{
			Label:         param.String(),
			Documentation: strings.Join(param.Comment, "\n"),
		})
	}

	res := &protocol.SignatureHelp{
		Signatures: []protocol.SignatureInformation{{
			Label:           fnName + targ.Function.String(),
			Documentation:   strings.Join(targ.Comment, "\n"),
			Parameters:      sigp,
			ActiveParameter: uint32(activeParam),
		}},
		ActiveParameter: uint32(activeParam),
		ActiveSignature: 0,
	}
	return res, nil
}

func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (result *protocol.Hover, err error) {
	resolver := s.NewResolver(params.TextDocument.URI)
	if resolver == nil {
		return &protocol.Hover{}, nil
	}

	node, _ := resolver.NodeAt(protoToPos(params.Position))
	if node == nil {
		return &protocol.Hover{}, nil
	}

	value := analysis.NodeToValue(node, resolver)
	var rnge *protocol.Range
	if value.Range.IsSet() {
		v := rangeToProto(value.Range)
		rnge = &v
	}

	doc := value.Type.String()
	if value.Function != nil {
		doc += value.Function.String()
	}
	if len(value.Comment) > 0 {
		doc += "\n"
		doc += strings.Join(value.Comment, "\n")
	}

	return &protocol.Hover{
		Range: rnge,
		Contents: protocol.MarkupContent{
			Kind:  protocol.PlainText,
			Value: doc,
		},
	}, nil
}

func (s *Server) Definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	resolver := s.NewResolver(params.TextDocument.URI)
	if resolver == nil {
		return []protocol.Location{}, nil
	}

	node, _ := resolver.NodeAt(protoToPos(params.Position))
	if node == nil {
		return []protocol.Location{}, nil
	}

	value := analysis.NodeToValue(node, resolver)
	if !value.Range.IsSet() {
		return []protocol.Location{}, nil
	}

	return []protocol.Location{{
		URI:   uri.File(value.Range.FileName),
		Range: rangeToProto(value.Range),
	}}, nil

}

func (s *Server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	current := s.overlay.Current(params.TextDocument.URI)
	if current == nil {
		return []protocol.TextEdit{}, nil
	}

	fname := params.TextDocument.URI.Filename()
	opts := s.config.FormatterOptions()
	if opts.Indent <= 0 {
		opts.Indent = int(params.Options.TabSize)
	}

	out, err := formatter.Format(fname, current.Contents, opts)
	if err != nil {
		return []protocol.TextEdit{}, nil
	}
    lines := uint32(strings.Count(current.Contents, "\n") + 1)
	return []protocol.TextEdit{{Range: protocol.Range{End: protocol.Position{Line: lines}}, NewText: string(out)}}, nil
}

type EvaluateParams struct {
	TextDocument *protocol.TextDocumentIdentifier `json:"textDocument"`
}

type EvaluateResult struct {
	Output string `json:"output"`
}

func formatRuntimeError(err error) string {
	rt, ok := err.(jsonnet.RuntimeError)
	if !ok {
		return err.Error()
	}
	sb := strings.Builder{}
	sb.WriteString(err.Error() + "\n")
	for _, frame := range rt.StackTrace {
		sb.WriteString(fmt.Sprintf("    %s: %s\n", frame.Loc.String(), frame.Name))
	}
	return sb.String()
}

func (s *Server) Evaluate(ctx context.Context, params *EvaluateParams) (*EvaluateResult, error) {
	cvm := s.getVM(params.TextDocument.URI)
	curAST := s.getCurrentAST(params.TextDocument.URI)
	if cvm == nil || curAST == nil {
		return nil, fmt.Errorf("cannot get jsonnet VM for file '%s'", params.TextDocument.URI.Filename())
	}

	result := &EvaluateResult{}
	var err error
	cvm.Use(func(vm *jsonnet.VM) {
		result.Output, err = vm.Evaluate(curAST)
		if err != nil {
			result.Output = formatRuntimeError(err)
		}
	})
	return result, nil
}

func (s *Server) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (result interface{}, err error) {
	if len(params.Arguments) != 1 {
		return nil, jsonrpc2.ErrInvalidParams
	}
	argData, ok := params.Arguments[0].(string)
	if !ok {
		return nil, jsonrpc2.ErrInvalidParams
	}

	switch params.Command {
	case "jsonnet.lsp.evaluate":
		args := &EvaluateParams{}
		if err := json.Unmarshal([]byte(argData), args); err != nil {
			return nil, jsonrpc2.ErrInvalidParams
		}
		return s.Evaluate(ctx, args)
	}

	return nil, jsonrpc2.ErrMethodNotFound
}
