package overlay

import (
	"fmt"
	"sort"
	"sync"

	"github.com/hexops/gotextdiff"
	"go.lsp.dev/uri"
)

type Entry struct {
	Contents string
	Version  int64
	// Custom user data attached to entry after parse() is called
	Data interface{}
}

type overlayFile struct {
	// This must be held when applying updates to the file
	updateLock sync.Mutex

	// Separately, these are held when getting/setting the entry pointers
	// named with underscore to make it more obvious they shouldn't be used directly
	entryLock sync.Mutex
	current   *Entry
	parsed    *Entry
}

type Overlay struct {
	updateLock  sync.Mutex
	updateQueue map[uri.URI][]fileUpdate

	fileLock sync.Mutex
	files    map[uri.URI]*overlayFile
}

type fileUpdate struct {
	URI     uri.URI
	Close   bool
	Version int64
	Replace *string
	Edits   []gotextdiff.TextEdit
}

func NewOverlay() *Overlay {
	return &Overlay{
		files:       map[uri.URI]*overlayFile{},
		updateQueue: map[uri.URI][]fileUpdate{},
	}
}

type UpdateResult struct {
	Current *Entry
	Parsed  *Entry
}

// UpdateFunc will be called at the end of an update batch. Calls to this callback
// for each individual file are linearized under a lock. Holding this lock may be useful
// for batching updates, f.ex waiting until diagnostics are done before returning.
type UpdateFunc func(UpdateResult)

// This function is used to parse the file that was just updated. It will be called for
// every item of a batch update. `lastEdit` may be set as the last edit made in the update.
type ParseFunc func(contents string, lastEdit *gotextdiff.TextEdit) (data interface{}, ok bool)

func EmptyParseFunc(string, *gotextdiff.TextEdit) (interface{}, bool) { return nil, true }

// applyFileUpdates run with ent.updateLock already locked
func applyFileUpdates(ent *overlayFile, updates []fileUpdate, parse ParseFunc) {
	for _, up := range updates {
		if up.Close {
			ent.entryLock.Lock()
			ent.current = nil
			ent.parsed = nil
			ent.entryLock.Unlock()
			continue
		}

		if replace := up.Replace; replace != nil {
			data, parsed := parse(*up.Replace, nil)
			res := &Entry{
				Contents: *up.Replace,
				Version:  up.Version,
				Data:     data,
			}

			ent.entryLock.Lock()
			ent.current = res
			if parsed {
				ent.parsed = res
			}
			ent.entryLock.Unlock()
			continue
		}

		// Delta Update

		// can we deal with these invariants better than panicing?
		// XXX: can we ask editor for updated version?
		if ent.current == nil {
			panic(fmt.Errorf("invariant: %s: delta-update for file with no data", up.URI.Filename()))
		}

		if up.Version != (ent.current.Version + 1) {
			panic(fmt.Errorf("invariant: %s: out of order delta-update for file with version current=%d new=%d", up.URI.Filename(), ent.current.Version, up.Version))
		}

		if len(up.Edits) == 0 {
			// in case of no updates, just change version and continue
			// make a new struct so we don't clobber in-use Entries
			parsedIsCurrent := ent.parsed != nil && ent.current.Version == ent.parsed.Version
			res := &Entry{
				Contents: ent.current.Contents,
				Version:  up.Version,
				Data:     ent.current.Data,
			}

			ent.entryLock.Lock()
			ent.current = res
			if parsedIsCurrent {
				ent.parsed = res
			}
			ent.entryLock.Unlock()
			continue
		}

		updated := gotextdiff.ApplyEdits(ent.current.Contents, up.Edits)
		lastEdit := up.Edits[len(up.Edits)-1]
		data, parsed := parse(updated, &lastEdit)

		res := &Entry{
			Contents: updated,
			Version:  up.Version,
			Data:     data,
		}

		ent.entryLock.Lock()
		ent.current = res
		if parsed {
			ent.parsed = res
		}
		ent.entryLock.Unlock()
	}
}

func (o *Overlay) Replace(u uri.URI, version int64, data string, parse ParseFunc, done UpdateFunc) {
	o.update(fileUpdate{URI: u, Version: version, Replace: &data}, parse, done)
}

func (o *Overlay) Close(u uri.URI) {
	o.update(fileUpdate{URI: u, Close: true}, EmptyParseFunc, func(UpdateResult) {})
}

func (o *Overlay) Update(u uri.URI, version int64, edits []gotextdiff.TextEdit, parse ParseFunc, done UpdateFunc) {
	o.update(fileUpdate{URI: u, Version: version, Edits: edits}, parse, done)
}

func (o *Overlay) Current(u uri.URI) *Entry {
	o.fileLock.Lock()
	ent := o.files[u]
	o.fileLock.Unlock()
	if ent == nil {
		return nil
	}
	ent.entryLock.Lock()
	defer ent.entryLock.Unlock()
	return ent.current
}

func (o *Overlay) Parsed(u uri.URI) *Entry {
	o.fileLock.Lock()
	ent := o.files[u]
	o.fileLock.Unlock()
	if ent == nil {
		return nil
	}
	ent.entryLock.Lock()
	defer ent.entryLock.Unlock()
	return ent.parsed
}

// getFile always returns non nil -- it will create an entry if it doesnt exist
func (o *Overlay) getFile(u uri.URI) *overlayFile {
	o.fileLock.Lock()
	f := o.files[u]
	if f == nil {
		f = &overlayFile{}
		o.files[u] = f
	}
	o.fileLock.Unlock()
	return f
}

func (o *Overlay) update(u fileUpdate, parse ParseFunc, done UpdateFunc) {
	// put the updates in queue as soon as possible to help make sure they are ordered
	o.updateLock.Lock()
	o.updateQueue[u.URI] = append(o.updateQueue[u.URI], u)
	o.updateLock.Unlock()

	// run asynchronously
	go func() {
		f := o.getFile(u.URI)

		// take out a lock on the file
		// do this to make sure we batch updates
		// if we take pending before this we could have a bunch of
		// goroutines waiting for small pending batches (which
		// would not be linearized).
		f.updateLock.Lock()
		defer f.updateLock.Unlock()

		o.updateLock.Lock()
		pending := o.updateQueue[u.URI]
		delete(o.updateQueue, u.URI)
		o.updateLock.Unlock()

		// another goroutine processed the updates
		if len(pending) == 0 {
			return
		}

		// if somehow a batch of updates came in out of order, try to remediate it
		sort.Slice(pending, func(i, j int) bool {
			return pending[i].Version < pending[j].Version
		})

		applyFileUpdates(f, pending, parse)

		// callback to user code
		// Note: this is intentionally called under lock to linearize updates
		// and allow user to control batching of things like diagnostics.
		done(UpdateResult{Current: f.current, Parsed: f.parsed})
	}()
}
