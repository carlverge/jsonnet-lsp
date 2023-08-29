package lsp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/carlverge/jsonnet-lsp/pkg/analysis"
	"github.com/carlverge/jsonnet-lsp/pkg/linter"
	"github.com/carlverge/jsonnet-lsp/pkg/overlay"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/span"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func logf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "I%s]%s\n", time.Now().Format("0201 15:04:05.00000"), fmt.Sprintf(msg, args...))
}

var traceEnable = true

func tracef(msg string, args ...interface{}) {
	if traceEnable {
		fmt.Fprintf(os.Stderr, "I%s]%s\n", time.Now().Format("0201 15:04:05.00000"), fmt.Sprintf(msg, args...))
	}
}

type Server struct {
	*FallbackServer

	rootURI     uri.URI
	rootFS      fs.FS
	searchPaths []string

	overlay  *overlay.Overlay
	importer *OverlayImporter
	vmlock   sync.Mutex
	config   *Configuration

	// intentionally only keep one active VM at once
	// when an operation needs a full VM (f.ex if it needs to
	// traverse imports) then dump the VM and create a new one.
	// This usually only happens when users switch and then edit a file,
	// and the latency is usually on the order of <1s. Not acceptable on
	// every operation, but acceptable on file change. This helps keep
	// memory usage low as we don't keep a VM in memory for every active
	// file we're editing.
	vm *vmCache

	// set to true if the last edit to the document was a '.'
	// used to change autocomplete behaviour
	lastCharIsDot bool

	cancel   context.CancelFunc
	notifier protocol.Client
}

type readCloser struct {
	io.ReadCloser
	io.Writer
}

func RunServer(ctx context.Context, stdout *os.File) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	conn := &readCloser{os.Stdin, stdout}

	logger := protocol.LoggerFromContext(ctx)
	logger.Debug("running in stdio mode")
	stream := jsonrpc2.NewStream(conn)
	jsonConn := jsonrpc2.NewConn(stream)
	notifier := protocol.ClientDispatcher(jsonConn, logger.Named("notify"))

	srv := &Server{
		FallbackServer: &FallbackServer{},
		overlay:        overlay.NewOverlay(),
		cancel:         cancel,
		notifier:       notifier,
		config:         &Configuration{},
	}

	handler := srv.Handler()
	jsonConn.Go(ctx, handler)

	select {
	case <-ctx.Done():
		_ = jsonConn.Close()
		return ctx.Err()
	case <-jsonConn.Done():
		if ctx.Err() == nil {
			if errors.Unwrap(jsonConn.Err()) != io.EOF {
				// only propagate connection error if context is still valid
				return jsonConn.Err()
			}
		}
	}

	return nil
}

func rootDirectoryFrom(params *protocol.InitializeParams) string {
	for _, f := range params.WorkspaceFolders {
		return f.URI
	}

	//lint:ignore SA1019 backwards compat
	if params.RootURI != "" {
		//lint:ignore SA1019 backwards compat
		return string(params.RootURI)
	}
	//lint:ignore SA1019 backwards compat
	if params.RootPath != "" {
		//lint:ignore SA1019 backwards compat
		return params.RootPath
	}
	cwd, _ := os.Getwd()
	return cwd
}

func findRootDirectory(params *protocol.InitializeParams) uri.URI {
	rootDir := rootDirectoryFrom(params)

	// The IntelliJ Bazel Plugin generates an artificial .ijwb project directory
	// inside the actual project root.
	// https://blog.bazel.build/2019/09/29/intellij-bazel-sync.html
	bazelDirs := []string{"/.ijwb", "/.aswb", "/.clwb"}
	for _, bazelDir := range bazelDirs {
		if strings.Contains(rootDir, bazelDir) {
			rootDir = strings.Replace(rootDir, bazelDir, "", 1)
			break
		}
	}
	return uri.URI(rootDir)
}

// cachedImporter will keep the file contents
// of each imported file stable. This is important for an LSP as
// file contents will change rather dynamically. The jsonnet VM
// will panic if it notices a file has changed underneath it.
type cachedImporter struct {
	lock     sync.Mutex
	notFound map[[2]string]error
	foundAt  map[[2]string]string
	cache    map[string]jsonnet.Contents
	real     jsonnet.Importer
}

func (imp *cachedImporter) Import(from, path string) (contents jsonnet.Contents, foundAt string, err error) {
	imp.lock.Lock()
	defer imp.lock.Unlock()

	key := [2]string{from, path}
	if foundAt, ok := imp.foundAt[key]; ok {
		return imp.cache[foundAt], foundAt, nil
	}

	if err, ok := imp.notFound[key]; ok {
		return jsonnet.Contents{}, "", err
	}

	contents, foundAt, err = imp.real.Import(from, path)
	if err != nil {
		imp.notFound[key] = err
		return contents, foundAt, err
	}

	imp.foundAt[key] = foundAt
	if _, ok := imp.cache[foundAt]; !ok {
		imp.cache[foundAt] = contents
	}
	// Always pull from the cache so we return the same value to jsonnet
	// if two imports hit the same file. Jsonnet will panic if we return
	// different contents, so this is critical.
	return imp.cache[foundAt], foundAt, nil
}

type OverlayImporter struct {
	overlay *overlay.Overlay
	rootURI uri.URI
	rootFS  fs.FS
	paths   []string

	// Additional user specified paths (can change at runtime)
	jpathLock sync.Mutex
	jpaths    []string
}

func (imp *OverlayImporter) readURI(uri uri.URI) (res []byte, err error) {
	// check overlay first -- use parsed as an unparsable result is not useful
	if ent := imp.overlay.Parsed(uri); ent != nil {
		return []byte(ent.Contents), nil
	}

	path, err := filepath.Rel(imp.rootURI.Filename(), uri.Filename())
	if err != nil {
		return nil, fmt.Errorf("failed to open URI '%s': %v", uri, err)
	}

	// TODO(@carlverge): More cruft with filesystem layout and importing.
	// If a search path is outside the workspace (and the rootFS we created)
	// then we can't open the file with the fs.FS functions.
	if filepath.IsAbs(uri.Filename()) && strings.HasPrefix(path, "../") {
		tracef("attempting import of file outside of workspace (root=%s): %s", imp.rootURI.Filename(), path)
		return os.ReadFile(uri.Filename())
	}

	defer func(t time.Time) {
		tracef("read file %s in %s (size=%d err=%v)", path, time.Since(t), len(res), err)
	}(time.Now())
	return fs.ReadFile(imp.rootFS, path)
}

func (imp *OverlayImporter) SetJPaths(jpaths []string) {
	imp.jpathLock.Lock()
	defer imp.jpathLock.Unlock()
	imp.jpaths = jpaths
}

func (imp *OverlayImporter) Import(from, path string) (jsonnet.Contents, string, error) {
	rootPath := imp.rootURI.Filename()

	// if absolute, rel it to the workspace root
	if filepath.IsAbs(path) {
		path, _ = filepath.Rel(rootPath, path)
	}

	// the path to the importer, relative to the root
	fromPath, err := filepath.Rel(rootPath, filepath.Dir(from))
	if err != nil {
		return jsonnet.Contents{}, "", fmt.Errorf("failed to open '%s' -- could not relativize '%s' to root '%s' %v", path, from, imp.rootURI, err)
	}

	// Build a list of candidate URIs to try for the file
	candidates := []uri.URI{
		uri.File(filepath.Join(rootPath, path)),
		uri.File(filepath.Join(rootPath, fromPath, path)),
	}
	for _, search := range imp.paths {
		candidates = append(candidates, uri.File(filepath.Join(rootPath, search, path)))
	}

	// JPaths feel very hacked in here.
	// They need to be reconfigurable at runtime.
	imp.jpathLock.Lock()
	jpaths := imp.jpaths
	imp.jpathLock.Unlock()
	for _, search := range jpaths {
		if filepath.IsAbs(search) {
			candidates = append(candidates, uri.File(filepath.Join(search, path)))
		} else {
			candidates = append(candidates, uri.File(filepath.Join(rootPath, search, path)))
		}
	}

	tracef("read-path: path='%s' from='%s' candidates=%v", path, from, candidates)
	tracef("searching for path '%s' in candidates %v", path, candidates)
	for _, candidate := range candidates {
		data, err := imp.readURI(candidate)
		if err == nil {
			tracef("read-path-hit: path='%s' foundAt=%s", path, candidate.Filename())
			return jsonnet.MakeContentsRaw(data), candidate.Filename(), nil
		}
	}
	return jsonnet.Contents{}, "", fmt.Errorf("path '%s' not found in candidates %v", path, candidates)
}

func posToProto(p ast.Location) protocol.Position {
	line, col := p.Line, p.Column
	if line > 0 {
		line--
	}
	if col > 0 {
		col--
	}
	return protocol.Position{Line: uint32(line), Character: uint32(col)}
}

func protoToPos(p protocol.Position) ast.Location {
	return ast.Location{Line: int(p.Line) + 1, Column: int(p.Character) + 1}
}

func rangeToProto(r ast.LocationRange) protocol.Range {
	return protocol.Range{Start: posToProto(r.Begin), End: posToProto(r.End)}
}

// staticError shadows the staticError internal interface in go-jsonnet/internal/errors
type staticError interface {
	Error() string
	Loc() ast.LocationRange
}

type ErrDiscard struct{}

func (e ErrDiscard) Format(err error) string                        { return err.Error() }
func (e ErrDiscard) SetMaxStackTraceSize(size int)                  {}
func (e ErrDiscard) SetColorFormatter(color jsonnet.ColorFormatter) {}

type vmCache struct {
	lock sync.Mutex
	// from is the file that created the VM
	from uri.URI
	vm   *jsonnet.VM
}

func (c *vmCache) Use(fn func(vm *jsonnet.VM)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	fn(c.vm)
}

func (c *vmCache) ImportAST(from, path string) (ast.Node, uri.URI) {
	c.lock.Lock()
	defer c.lock.Unlock()
	contents, foundAt, err := c.vm.ImportAST(from, path)
	if err != nil {
		return nil, uri.URI("")
	}
	return contents, uri.File(foundAt)
}

func (s *Server) getVM(uri uri.URI) *vmCache {
	s.vmlock.Lock()
	defer s.vmlock.Unlock()

	// still on the same file, keep the vm cache
	if s.vm != nil && uri == s.vm.from {
		return s.vm
	}

	tracef("flusing jsonnet vm cache (changed file to %s)", uri)
	vm := &vmCache{from: uri, vm: jsonnet.MakeVM()}
	vm.vm.Importer(&cachedImporter{
		notFound: map[[2]string]error{},
		foundAt:  map[[2]string]string{},
		cache:    map[string]jsonnet.Contents{},
		real:     s.importer,
	})
	vm.vm.SetTraceOut(io.Discard)
	s.vm = vm

	return vm
}

func convChangeEvents(events []protocol.TextDocumentContentChangeEvent) []gotextdiff.TextEdit {
	res := make([]gotextdiff.TextEdit, len(events))
	for i, ev := range events {
		res[i] = gotextdiff.TextEdit{
			Span: span.New(
				span.URI(""),
				span.NewPoint(int(ev.Range.Start.Line)+1, int(ev.Range.Start.Character)+1, -1),
				span.NewPoint(int(ev.Range.End.Line)+1, int(ev.Range.End.Character)+1, -1),
			),
			NewText: ev.Text,
		}
	}
	return res
}

type ParseResult struct {
	Root ast.Node
	Err  error
}

func (p *ParseResult) StaticErr() staticError {
	if p == nil {
		return nil
	}
	se, _ := p.Err.(staticError)
	return se
}

// AST recovery. Unfortunately jsonnet makes significant use of semicolons and colons for valid ASTs.
// As a user is typing, the AST will often be invalid due to a missing semicolon or comma.
// For example: `local x = std` -- when the user hits '.' autocomplete wont work because the
// previous AST could not be parsed (`local x = std;` is valid, however).
// The code below will try to add a semicolon and a comma to the text, the character after
// where the user is typing.
// We need still need to set the original AST error so it will be reported.
func tryRecoverAST(uri uri.URI, contents string, lastEdit *gotextdiff.TextEdit) ast.Node {
	// Eat panics from textedit
	defer func() { _ = recover() }()
	insertion := span.NewPoint(lastEdit.Span.End().Line(), lastEdit.Span.End().Column()+len(lastEdit.NewText), -1)
	addSemicol := []gotextdiff.TextEdit{{NewText: ";", Span: span.New(span.URI(""), insertion, insertion)}}
	addComma := []gotextdiff.TextEdit{{NewText: ",", Span: span.New(span.URI(""), insertion, insertion)}}

	withSemicol := gotextdiff.ApplyEdits(contents, addSemicol)
	if recovered, _ := jsonnet.SnippetToAST(uri.Filename(), withSemicol); recovered != nil {
		return recovered
	}

	withComma := gotextdiff.ApplyEdits(contents, addComma)
	if recovered, _ := jsonnet.SnippetToAST(uri.Filename(), withComma); recovered != nil {
		return recovered
	}

	return nil
}

func parseJsonnetFn(uri uri.URI) overlay.ParseFunc {
	return func(contents string, lastEdit *gotextdiff.TextEdit) (result interface{}, success bool) {
		defer func(t time.Time) { tracef("parsed ast uri=%s len=%d in %s", uri, len(contents), time.Since(t)) }(time.Now())
		res := &ParseResult{}
		res.Root, res.Err = jsonnet.SnippetToAST(uri.Filename(), contents)

		if res.Root == nil && lastEdit != nil {
			res.Root = tryRecoverAST(uri, contents, lastEdit)
		}

		return res, res.Root != nil
	}
}

func (s *Server) processFileUpdateFn(ctx context.Context, uri uri.URI) overlay.UpdateFunc {
	resv := &valueResolver{
		rootURI:    uri,
		rootAST:    nil,
		roots:      map[string]ast.Node{},
		stackCache: map[ast.Node][]ast.Node{},
		getvm:      func() *vmCache { return s.getVM(uri) },
	}

	diags := []protocol.Diagnostic{}
	return func(ur overlay.UpdateResult) {
		defer func(t time.Time) { tracef("linting %s done diags in %s", uri, time.Since(t)) }(time.Now())
		if ur.Current == nil {
			return
		}

		if pr, _ := ur.Current.Data.(*ParseResult); pr.StaticErr() != nil {
			// AST failed to parse, do not run lints
			se := pr.StaticErr()
			diags = append(diags, protocol.Diagnostic{
				Severity: protocol.DiagnosticSeverityError,
				Range:    rangeToProto(se.Loc()),
				Message:  se.Error(),
				Source:   "jsonnet",
			})
		} else if ur.Parsed != nil && s.config.Diag.Linter && ur.Current.Version == ur.Parsed.Version {
			// AST did parse, run linter
			parseResult := ur.Parsed.Data.(*ParseResult)
			resv.rootAST = parseResult.Root
			resv.roots[resv.rootAST.Loc().FileName] = resv.rootAST
			diags = append(diags, linter.LintAST(resv.rootAST, resv)...)

			// If the linter has detected no fatal errors, then evaluate the file.
			// This is to avoid evaluations of obviously bad files, which will just
			// burn CPU as the user is typing.
			if !linter.HasErrors(diags) && s.config.Diag.Evaluate {
				resv.getvm().Use(func(vm *jsonnet.VM) {
					defer func(t time.Time) { tracef("evaluation %s done diags in %s", uri, time.Since(t)) }(time.Now())
					_, err := vm.Evaluate(resv.rootAST)
					rterr, ok := err.(jsonnet.RuntimeError)
					if !ok {
						return
					}

					// Grab the stack trace from the error, and highlight
					// each line.
					fname := resv.rootAST.Loc().FileName
					seenRootCause := false
					for _, frame := range rterr.StackTrace {
						if frame.Loc.FileName != fname {
							continue
						}
						// Each implicated line of the stack trace is a diagnostic to be highlighted.
						// The most specific stack frame in this file is highlighted as an error
						// to draw user attention to the clostest known root cause.
						sev := protocol.DiagnosticSeverityError
						if seenRootCause {
							sev = protocol.DiagnosticSeverityWarning
						}
						seenRootCause = true

						diags = append(diags, protocol.Diagnostic{
							Range:    rangeToProto(frame.Loc),
							Severity: sev,
							Code:     "RuntimeError",
							Source:   "jsonnet",
							Message:  rterr.Msg,
						})
					}
				})
			}
		}

		_ = s.notifier.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
			URI:         uri,
			Version:     uint32(ur.Current.Version),
			Diagnostics: diags,
		})
	}
}

type valueResolver struct {
	rootURI uri.URI
	rootAST ast.Node
	// A map of filenames from node.Loc().Filename to the root AST node
	// This is used to find the root AST node of any node.
	stackCache map[ast.Node][]ast.Node
	roots      map[string]ast.Node
	getvm      func() *vmCache
	vm         *vmCache
}

var _ = (analysis.Resolver)(new(valueResolver))

func (s *Server) NewResolver(uri uri.URI) *valueResolver {
	root := s.getCurrentAST(uri)
	if root == nil {
		return nil
	}
	return &valueResolver{
		rootURI:    uri,
		rootAST:    root,
		roots:      map[string]ast.Node{root.Loc().FileName: root},
		stackCache: map[ast.Node][]ast.Node{},
		getvm:      func() *vmCache { return s.getVM(uri) },
	}
}

func (r *valueResolver) NodeAt(loc ast.Location) (node ast.Node, stack []ast.Node) {
	stack = analysis.StackAtLoc(r.rootAST, loc)
	if len(stack) == 0 {
		return nil, nil
	}
	node = stack[len(stack)-1]
	r.stackCache[node] = stack
	return node, stack
}

func (r *valueResolver) Vars(from ast.Node) analysis.VarMap {
	if from == nil || from.Loc() == nil {
		return analysis.VarMap{}
	}
	root := r.roots[from.Loc().FileName]
	if root == nil {
		panic(fmt.Errorf("invariant: resolving var from %s where no root was imported", analysis.FmtNode(from)))
	}
	if stk := r.stackCache[from]; len(stk) > 0 {
		return analysis.StackVars(stk)
	}
	stk := analysis.StackAtNode(root, from)
	return analysis.StackVars(stk)
}

func (r *valueResolver) Import(from, path string) ast.Node {
	// The reason for this dance is to only grab a VM and importer
	// if we need to import something. This allows us to avoid thrashing the
	// vm cache when we don't actually need a full VM to perform analysis
	if r.vm == nil {
		if r.getvm == nil {
			return nil
		}
		r.vm = r.getvm()
	}
	root, _ := r.vm.ImportAST(from, path)
	if root != nil {
		r.roots[root.Loc().FileName] = root
	}
	return root
}

func (s *Server) getCurrentAST(uri uri.URI) ast.Node {
	parsed := s.overlay.Parsed(uri)
	if parsed == nil {
		return nil
	}
	res, _ := parsed.Data.(*ParseResult)
	if res == nil || res.Root == nil {
		return nil
	}
	return res.Root
}
