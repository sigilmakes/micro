package buffer

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	luar "layeh.com/gopher-luar"

	"github.com/micro-editor/micro/v2/internal/config"
	ulua "github.com/micro-editor/micro/v2/internal/lua"
	"github.com/micro-editor/micro/v2/internal/screen"
	"github.com/micro-editor/micro/v2/internal/util"
	"github.com/micro-editor/micro/v2/pkg/filetype"
	"github.com/micro-editor/micro/v2/pkg/syntax"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	// OpenBuffers is a list of the currently open buffers
	OpenBuffers  []*Buffer
	nextBufferID uint64
	// LogBuf is a reference to the log buffer which can be opened with the
	// `> log` command
	LogBuf *Buffer
)

// The BufType defines what kind of buffer this is
type BufType struct {
	Kind     int
	Readonly bool // The buffer cannot be edited
	Scratch  bool // The buffer cannot be saved
	Syntax   bool // Syntax highlighting is enabled
}

var (
	// BTDefault is a default buffer
	BTDefault = BufType{0, false, false, true}
	// BTHelp is a help buffer
	BTHelp = BufType{1, true, true, true}
	// BTLog is a log buffer
	BTLog = BufType{2, true, true, false}
	// BTScratch is a buffer that cannot be saved (for scratch work)
	BTScratch = BufType{3, false, true, false}
	// BTRaw is a buffer that shows raw terminal events
	BTRaw = BufType{4, false, true, false}
	// BTInfo is a buffer for inputting information
	BTInfo = BufType{5, false, true, false}
	// BTStdout is a buffer that only writes to stdout
	// when closed
	BTStdout = BufType{6, false, true, true}
)

// SharedBuffer is a struct containing info that is shared among buffers
// that have the same file open
type SharedBuffer struct {
	*LineArray
	// Stores the last modification time of the file the buffer is pointing to
	ModTime time.Time
	// Type of the buffer (e.g. help, raw, scratch etc..)
	Type BufType

	// Path to the file on disk
	Path string
	// Absolute path to the file on disk
	AbsPath string
	// Name of the buffer on the status line
	name string

	toStdout bool

	// Settings customized by the user
	Settings map[string]any
	// LocalSettings customized by the user for this buffer only
	LocalSettings map[string]bool

	encoding encoding.Encoding

	Suggestions   []string
	Completions   []string
	CurSuggestion int

	Messages []*Message

	updateDiffTimer   *time.Timer
	diffBase          []byte
	diffBaseLineCount int
	diffLock          sync.RWMutex
	diff              map[int]DiffStatus

	forceKeepBackup bool

	// ReloadDisabled allows the user to disable reloads if they
	// are viewing a file that is constantly changing
	ReloadDisabled bool

	isModified bool
	// Whether or not suggestions can be autocompleted must be shared because
	// it changes based on how the buffer has changed
	HasSuggestions bool

	hl    *highlightSession
	hlGen uint64
	hlID  uint64

	ModifiedThisFrame bool

	// Hash of the original buffer -- empty if fastdirty is on
	origHash [md5.Size]byte
}

func (b *SharedBuffer) insert(pos Loc, value []byte) {
	b.HasSuggestions = false
	b.LineArray.insert(pos, value)
	b.setModified()

	inslines := bytes.Count(value, []byte{'\n'})
	b.recordEdit(syntax.Edit{
		StartLine:  pos.Y,
		StartCol:   pos.X,
		OldEndLine: pos.Y,
		OldEndCol:  pos.X,
		NewEndLine: pos.Y + inslines,
		NewEndCol:  insertedEndCol(pos, value, inslines),
	})
	b.MarkModified(pos.Y, pos.Y+inslines)
}

func (b *SharedBuffer) remove(start, end Loc) []byte {
	b.HasSuggestions = false
	defer b.setModified()
	defer b.MarkModified(start.Y, end.Y)
	b.recordEdit(syntax.Edit{
		StartLine:  start.Y,
		StartCol:   start.X,
		OldEndLine: end.Y,
		OldEndCol:  end.X,
		NewEndLine: start.Y,
		NewEndCol:  start.X,
	})
	return b.LineArray.remove(start, end)
}

func insertedEndCol(pos Loc, value []byte, newlines int) int {
	if newlines == 0 {
		return pos.X + syntax.CharacterCount(value)
	}
	last := bytes.LastIndexByte(value, '\n')
	return syntax.CharacterCount(value[last+1:])
}

func (b *SharedBuffer) setModified() {
	if b.Type.Scratch {
		return
	}

	if b.Settings["fastdirty"].(bool) {
		b.isModified = true
	} else {
		var buff [md5.Size]byte

		b.calcHash(&buff)
		b.isModified = buff != b.origHash
	}

	if b.isModified {
		b.RequestBackup()
	} else {
		b.CancelBackup()
	}
}

// calcHash calculates md5 hash of all lines in the buffer
func (b *SharedBuffer) calcHash(out *[md5.Size]byte) {
	h := md5.New()

	if len(b.lines) > 0 {
		h.Write(b.lines[0].data)

		for _, l := range b.lines[1:] {
			if b.Endings == FFDos {
				h.Write([]byte{'\r', '\n'})
			} else {
				h.Write([]byte{'\n'})
			}
			h.Write(l.data)
		}
	}

	h.Sum((*out)[:0])
}

// MarkModified marks the buffer as modified for this frame and asks the
// active highlighting provider for fresh spans after a short debounce.
func (b *SharedBuffer) MarkModified(start, end int) {
	b.ModifiedThisFrame = true

	start = util.Clamp(start, 0, len(b.lines)-1)
	end = util.Clamp(end, 0, len(b.lines)-1)

	if b.hl != nil {
		b.hlGen++
		b.hl.cause = syntax.CauseEdit
		b.scheduleHighlight()
	}

	for i := start; i <= end; i++ {
		b.LineArray.invalidateSearchMatches(i)
	}
}

// DisableReload disables future reloads of this sharedbuffer
func (b *SharedBuffer) DisableReload() {
	b.ReloadDisabled = true
}

const (
	DSUnchanged    = 0
	DSAdded        = 1
	DSModified     = 2
	DSDeletedAbove = 3
)

type DiffStatus byte

type Command struct {
	StartCursor      Loc
	SearchRegex      string
	SearchAfterStart bool
}

var emptyCommand = Command{
	StartCursor:      Loc{-1, -1},
	SearchRegex:      "",
	SearchAfterStart: false,
}

// Buffer stores the main information about a currently open file including
// the actual text (in a LineArray), the undo/redo stack (in an EventHandler)
// all the cursors, the syntax highlighting info, the settings for the buffer
// and some misc info about modification time and path location.
// The syntax highlighting info must be stored with the buffer because the syntax
// highlighter attaches information to each line of the buffer for optimization
// purposes so it doesn't have to rehighlight everything on every update.
// Likewise for the search highlighting.
type Buffer struct {
	*EventHandler
	*SharedBuffer

	cursors     []*Cursor
	curCursor   int
	StartCursor Loc

	// OptionCallback is called after a buffer option value is changed.
	// The display module registers its OptionCallback to ensure the buffer window
	// is properly updated when needed. This is a workaround for the fact that
	// the buffer module cannot directly call the display's API (it would mean
	// a circular dependency between packages).
	OptionCallback func(option string, nativeValue any)

	// The display module registers its own GetVisualX function for getting
	// the correct visual x location of a cursor when softwrap is used.
	// This is hacky. Maybe it would be better to move all the visual x logic
	// from buffer to display, but it would require rewriting a lot of code.
	GetVisualX func(loc Loc) int

	// Last search stores the last successful search
	LastSearch      string
	LastSearchRegex bool
	// HighlightSearch enables highlighting all instances of the last successful search
	HighlightSearch bool

	// OverwriteMode indicates that we are in overwrite mode (toggled by
	// Insert key by default) i.e. that typing a character shall replace the
	// character under the cursor instead of inserting a character before it.
	OverwriteMode bool
}

// NewBufferFromFileWithCommand opens a new buffer with a given command
// If cmd.StartCursor is {-1, -1} the location does not overwrite what the cursor location
// would otherwise be (start of file, or saved cursor position if `savecursor` is
// enabled)
func NewBufferFromFileWithCommand(path string, btype BufType, cmd Command) (*Buffer, error) {
	var err error
	filename := path
	if config.GetGlobalOption("parsecursor").(bool) && cmd.StartCursor.X == -1 && cmd.StartCursor.Y == -1 {
		var cursorPos []string
		filename, cursorPos = util.GetPathAndCursorPosition(filename)
		cmd.StartCursor, err = ParseCursorLocation(cursorPos)
		if err != nil {
			cmd.StartCursor = Loc{-1, -1}
		}
	}

	filename, err = util.ReplaceHome(filename)
	if err != nil {
		return nil, err
	}

	fileInfo, serr := os.Stat(filename)
	if serr != nil && !errors.Is(serr, fs.ErrNotExist) {
		return nil, serr
	}
	if serr == nil && fileInfo.IsDir() {
		return nil, errors.New("Error: " + filename + " is a directory and cannot be opened")
	}
	if serr == nil && !fileInfo.Mode().IsRegular() {
		return nil, errors.New("Error: " + filename + " is not a regular file and cannot be opened")
	}

	f, err := os.OpenFile(filename, os.O_WRONLY, 0)
	readonly := errors.Is(err, fs.ErrPermission)
	f.Close()

	file, err := os.Open(filename)
	if err == nil {
		defer file.Close()
	}

	var buf *Buffer
	if errors.Is(err, fs.ErrNotExist) {
		// File does not exist -- create an empty buffer with that name
		buf = NewBufferFromString("", filename, btype)
	} else if err != nil {
		return nil, err
	} else {
		buf = NewBuffer(file, util.FSize(file), filename, btype, cmd)
		if buf == nil {
			return nil, errors.New("could not open file")
		}
	}

	if readonly && prompt != nil {
		prompt.Message(fmt.Sprintf("Warning: file is readonly - %s will be attempted when saving", config.GlobalSettings["sucmd"].(string)))
		// buf.SetOptionNative("readonly", true)
	}

	return buf, nil
}

// NewBufferFromFile opens a new buffer using the given path
// It will also automatically handle `~`, and line/column with filename:l:c
// It will return an empty buffer if the path does not exist
// and an error if the file is a directory
func NewBufferFromFile(path string, btype BufType) (*Buffer, error) {
	return NewBufferFromFileWithCommand(path, btype, emptyCommand)
}

// NewBufferFromStringWithCommand creates a new buffer containing the given string
// with a cursor loc and a search text
func NewBufferFromStringWithCommand(text, path string, btype BufType, cmd Command) *Buffer {
	return NewBuffer(strings.NewReader(text), int64(len(text)), path, btype, cmd)
}

// NewBufferFromString creates a new buffer containing the given string
func NewBufferFromString(text, path string, btype BufType) *Buffer {
	return NewBuffer(strings.NewReader(text), int64(len(text)), path, btype, emptyCommand)
}

// NewBuffer creates a new buffer from a given reader with a given path
// Ensure that ReadSettings and InitGlobalSettings have been called before creating
// a new buffer
// Places the cursor at startcursor. If startcursor is -1, -1 places the
// cursor at an autodetected location (based on savecursor or :LINE:COL)
func NewBuffer(r io.Reader, size int64, path string, btype BufType, cmd Command) *Buffer {
	absPath := path
	if btype == BTDefault && path != "" {
		absPath = util.ResolvePath(path)
	}

	b := new(Buffer)

	found := false
	if len(path) > 0 {
		for _, buf := range OpenBuffers {
			if buf.AbsPath == absPath && buf.Type != BTInfo {
				found = true
				b.SharedBuffer = buf.SharedBuffer
				b.EventHandler = buf.EventHandler
			}
		}
	}

	hasBackup := false
	if !found {
		b.SharedBuffer = new(SharedBuffer)
		b.hlID = atomic.AddUint64(&nextBufferID, 1)
		b.Type = btype

		b.AbsPath = absPath
		b.Path = path

		b.Settings = config.DefaultCommonSettings()
		b.LocalSettings = make(map[string]bool)
		for k, v := range config.GlobalSettings {
			if _, ok := config.DefaultGlobalOnlySettings[k]; !ok {
				// make sure setting is not global-only
				b.Settings[k] = v
			}
		}
		config.UpdatePathGlobLocals(b.Settings, absPath)

		var err error
		b.encoding, err = htmlindex.Get(b.Settings["encoding"].(string))
		if err != nil {
			b.encoding = unicode.UTF8
			b.Settings["encoding"] = "utf-8"
		}

		var ok bool
		hasBackup, ok = b.ApplyBackup(size)

		if !ok {
			return NewBufferFromString("", "", btype)
		}
		if !hasBackup {
			reader := bufio.NewReader(transform.NewReader(r, b.encoding.NewDecoder()))

			var ff FileFormat = FFAuto

			if size == 0 {
				// for empty files, use the fileformat setting instead of
				// autodetection
				switch b.Settings["fileformat"] {
				case "unix":
					ff = FFUnix
				case "dos":
					ff = FFDos
				}
			} else {
				// in case of autodetection treat as locally set
				b.LocalSettings["fileformat"] = true
			}

			b.LineArray = NewLineArray(uint64(size), ff, reader)
		}
		b.EventHandler = NewEventHandler(b.SharedBuffer, b.cursors)

		// The last time this file was modified
		b.UpdateModTime()
	}

	if b.Settings["readonly"].(bool) && b.Type == BTDefault {
		b.Type.Readonly = true
	}

	switch b.Endings {
	case FFUnix:
		b.Settings["fileformat"] = "unix"
	case FFDos:
		b.Settings["fileformat"] = "dos"
	}

	b.UpdateRules()
	// we know the filetype now, so update per-filetype settings
	config.UpdateFileTypeLocals(b.Settings, b.Settings["filetype"].(string))

	if _, err := os.Stat(filepath.Join(config.ConfigDir, "buffers")); errors.Is(err, fs.ErrNotExist) {
		os.Mkdir(filepath.Join(config.ConfigDir, "buffers"), os.ModePerm)
	}

	if cmd.StartCursor.X != -1 && cmd.StartCursor.Y != -1 {
		b.StartCursor = cmd.StartCursor
	} else if b.Settings["savecursor"].(bool) || b.Settings["saveundo"].(bool) {
		err := b.Unserialize()
		if err != nil {
			screen.TermMessage(err)
		}
	}

	b.AddCursor(NewCursor(b, b.StartCursor))
	b.GetActiveCursor().Relocate()

	if cmd.SearchRegex != "" {
		match, found, _ := b.FindNext(cmd.SearchRegex, b.Start(), b.End(), b.StartCursor, true, true)
		if found {
			if cmd.SearchAfterStart {
				// Search from current cursor and move it accordingly
				b.GetActiveCursor().SetSelectionStart(match[0])
				b.GetActiveCursor().SetSelectionEnd(match[1])
				b.GetActiveCursor().OrigSelection[0] = b.GetActiveCursor().CurSelection[0]
				b.GetActiveCursor().OrigSelection[1] = b.GetActiveCursor().CurSelection[1]
				b.GetActiveCursor().GotoLoc(match[1])
			}
			b.LastSearch = cmd.SearchRegex
			b.LastSearchRegex = true
			b.HighlightSearch = b.Settings["hlsearch"].(bool)
		}
	}

	if !b.Settings["fastdirty"].(bool) && !found {
		if size > LargeFileThreshold {
			// If the file is larger than LargeFileThreshold fastdirty needs to be on
			b.Settings["fastdirty"] = true
		} else if !hasBackup {
			// since applying a backup does not save the applied backup to disk, we should
			// not calculate the original hash based on the backup data
			b.calcHash(&b.origHash)
		}
	}

	err := config.RunPluginFn("onBufferOpen", luar.New(ulua.L, b))
	if err != nil {
		screen.TermMessage(err)
	}

	OpenBuffers = append(OpenBuffers, b)

	return b
}

// CloseOpenBuffers removes all open buffers
func CloseOpenBuffers() {
	// use a copy, since we're gonna remove elements from OpenBuffers
	// while iterating
	buffers := make([]*Buffer, len(OpenBuffers))
	copy(buffers, OpenBuffers)

	for _, buf := range buffers {
		buf.Close()
	}
}

// Close removes this buffer from the list of open buffers
func (b *Buffer) Close() {
	for i, buf := range OpenBuffers {
		if b == buf {
			b.Fini()
			copy(OpenBuffers[i:], OpenBuffers[i+1:])
			OpenBuffers[len(OpenBuffers)-1] = nil
			OpenBuffers = OpenBuffers[:len(OpenBuffers)-1]
			return
		}
	}
}

// Fini should be called when a buffer is closed and performs
// some cleanup
func (b *Buffer) Fini() {
	if !b.Shared() {
		if !b.Modified() {
			b.Serialize()
		}
		b.CancelBackup()
		b.SharedBuffer.closeHighlight()
	}

	if b.Type == BTStdout {
		fmt.Fprint(util.Stdout, string(b.Bytes()))
	}
}

// GetName returns the name that should be displayed in the statusline
// for this buffer
func (b *Buffer) GetName() string {
	name := b.name
	if name == "" {
		if b.Path == "" {
			return "No name"
		}
		name = b.Path
	}
	if b.Settings["basename"].(bool) {
		return filepath.Base(name)
	}
	return name
}

// SetName changes the name for this buffer
func (b *Buffer) SetName(s string) {
	b.name = s
}

// Insert inserts the given string of text at the start location
func (b *Buffer) Insert(start Loc, text string) {
	if !b.Type.Readonly {
		b.EventHandler.cursors = b.cursors
		b.EventHandler.active = b.curCursor
		b.EventHandler.Insert(start, text)
	}
}

// Remove removes the characters between the start and end locations
func (b *Buffer) Remove(start, end Loc) {
	if !b.Type.Readonly {
		b.EventHandler.cursors = b.cursors
		b.EventHandler.active = b.curCursor
		b.EventHandler.Remove(start, end)
	}
}

// FileType returns the buffer's filetype
func (b *Buffer) FileType() string {
	return b.Settings["filetype"].(string)
}

// ExternallyModified returns whether the file being edited has
// been modified by some external process
func (b *Buffer) ExternallyModified() bool {
	modTime, err := util.GetModTime(b.Path)
	if err == nil {
		return modTime != b.ModTime
	}
	return false
}

// UpdateModTime updates the modtime of this file
func (b *Buffer) UpdateModTime() (err error) {
	b.ModTime, err = util.GetModTime(b.Path)
	return
}

// ReOpen reloads the current buffer from disk
func (b *Buffer) ReOpen() error {
	file, err := os.Open(b.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc, err := htmlindex.Get(b.Settings["encoding"].(string))
	if err != nil {
		return err
	}

	reader := bufio.NewReader(transform.NewReader(file, enc.NewDecoder()))
	data, err := io.ReadAll(reader)
	txt := string(data)

	if err != nil {
		return err
	}
	b.EventHandler.ApplyDiff(txt)

	err = b.UpdateModTime()
	if !b.Settings["fastdirty"].(bool) {
		if len(data) > LargeFileThreshold {
			b.Settings["fastdirty"] = true
		} else {
			b.calcHash(&b.origHash)
		}
	}
	b.isModified = false
	b.RelocateCursors()
	return err
}

// RelocateCursors relocates all cursors (makes sure they are in the buffer)
func (b *Buffer) RelocateCursors() {
	for _, c := range b.cursors {
		c.Relocate()
	}
}

// DeselectCursors removes selection from all cursors
func (b *Buffer) DeselectCursors() {
	for _, c := range b.cursors {
		c.Deselect(true)
	}
}

// RuneAt returns the rune at a given location in the buffer
func (b *Buffer) RuneAt(loc Loc) rune {
	line := b.LineBytes(loc.Y)
	if len(line) > 0 {
		i := 0
		for len(line) > 0 {
			r, _, size := util.DecodeCharacter(line)
			line = line[size:]

			if i == loc.X {
				return r
			}

			i++
		}
	}
	return '\n'
}

// WordAt returns the word around a given location in the buffer
func (b *Buffer) WordAt(loc Loc) []byte {
	if len(b.LineBytes(loc.Y)) == 0 || !util.IsWordChar(b.RuneAt(loc)) {
		return []byte{}
	}

	start := loc
	end := loc.Move(1, b)

	for start.X > 0 && util.IsWordChar(b.RuneAt(start.Move(-1, b))) {
		start.X--
	}

	lineLen := util.CharacterCount(b.LineBytes(loc.Y))
	for end.X < lineLen && util.IsWordChar(b.RuneAt(end)) {
		end.X++
	}

	return b.Substr(start, end)
}

// Shared returns if there are other buffers with the same file as this buffer
func (b *Buffer) Shared() bool {
	for _, buf := range OpenBuffers {
		if buf != b && buf.SharedBuffer == b.SharedBuffer {
			return true
		}
	}
	return false
}

// Modified returns if this buffer has been modified since
// being opened
func (b *Buffer) Modified() bool {
	return b.isModified
}

// Size returns the number of bytes in the current buffer
func (b *Buffer) Size() int {
	nb := 0
	for i := 0; i < b.LinesNum(); i++ {
		nb += len(b.LineBytes(i))

		if i != b.LinesNum()-1 {
			if b.Endings == FFDos {
				nb++ // carriage return
			}
			nb++ // newline
		}
	}
	return nb
}

// HighlightDebounce is how long edits settle before a provider request.
// It is a variable so tests can shorten it.
var HighlightDebounce = 25 * time.Millisecond

// MainThread schedules f on the editor's main goroutine. cmd/micro replaces
// this with a send to its timer channel so Lua providers never run on a
// timer goroutine.
var MainThread = func(f func()) { f() }

// plainMatch is shared by unstyled lines. It is never mutated after init.
var plainMatch = syntax.LineMatch{0: syntax.NoGroup}

type highlightSession struct {
	provider   *syntax.Provider
	filetype   string
	requestGen uint64
	inFlight   bool
	dirty      bool
	closed     bool
	cause      syntax.Cause
	timer      *time.Timer
	edits      []syntax.Edit
}

func (b *SharedBuffer) recordEdit(e syntax.Edit) {
	if b.hl != nil && !b.hl.closed {
		b.hl.edits = append(b.hl.edits, e)
	}
}

func (b *SharedBuffer) highlightView() *Buffer {
	for _, buf := range OpenBuffers {
		if buf.SharedBuffer == b {
			return buf
		}
	}
	return nil
}

// Text returns the whole buffer as UTF-8 lines joined with a single newline.
func (b *SharedBuffer) Text() string {
	var sb strings.Builder
	for i := 0; i < b.LinesNum(); i++ {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.Write(b.LineBytes(i))
	}
	return sb.String()
}

// UpdateRules is kept as an alias for plugins that call buf:UpdateRules().
func (b *Buffer) UpdateRules() {
	b.UpdateProvider()
}

// UpdateProvider re-detects the filetype and (re)attaches a highlighting
// provider. No provider is a supported state: filetype identity still works.
func (b *Buffer) UpdateProvider() {
	if b.SharedBuffer == nil {
		return
	}
	b.SharedBuffer.updateProvider()
}

func (b *SharedBuffer) updateProvider() {
	if !b.Type.Syntax {
		b.detachHighlight(true)
		return
	}

	ft := b.Settings["filetype"].(string)
	if ft == "off" {
		b.detachHighlight(true)
		return
	}

	detectLimit := util.IntOpt(b.Settings["detectlimit"])
	if ft == "" || ft == "unknown" {
		if detected := filetype.Match(b.Path, b, detectLimit); detected != "" {
			ft = detected
		}
	}

	if p := syntax.Lookup(b.Path, ft, b, detectLimit); p != nil {
		if p.Match.FileType != "" {
			ft = p.Match.FileType
		}
		b.Settings["filetype"] = ft
		if b.Settings["syntax"].(bool) {
			b.attachHighlight(p, ft)
			return
		}
	} else if ft != "" && ft != b.Settings["filetype"] {
		b.Settings["filetype"] = ft
	}

	b.detachHighlight(true)
}

func (b *SharedBuffer) attachHighlight(p *syntax.Provider, ft string) {
	if b.hl != nil && !b.hl.closed && b.hl.provider == p && b.hl.filetype == ft {
		return
	}
	b.detachHighlight(false)
	b.hlGen++
	b.hl = &highlightSession{
		provider: p,
		filetype: ft,
		cause:    syntax.CauseOpen,
	}
	b.ClearMatches()
	b.scheduleHighlight()
}

func (b *SharedBuffer) detachHighlight(clear bool) {
	if h := b.hl; h != nil {
		if h.timer != nil {
			h.timer.Stop()
			h.timer = nil
		}
		h.closed = true
		b.hl = nil
	}
	b.hlGen++
	if clear {
		b.ClearMatches()
		screen.Redraw()
	}
}

func (b *SharedBuffer) scheduleHighlight() {
	h := b.hl
	if h == nil || h.closed {
		return
	}
	h.dirty = true
	if h.timer != nil {
		return
	}
	run := MainThread
	h.timer = time.AfterFunc(HighlightDebounce, func() {
		run(func() { b.highlightTick() })
	})
}

func (b *SharedBuffer) highlightTick() {
	h := b.hl
	if h == nil || h.closed {
		return
	}
	h.timer = nil
	if !h.dirty || h.inFlight {
		return
	}
	if !b.Settings["syntax"].(bool) || h.provider == nil {
		h.dirty = false
		return
	}

	view := b.highlightView()
	if view == nil {
		h.dirty = false
		return
	}

	h.dirty = false
	h.inFlight = true
	h.requestGen = b.hlGen
	req := syntax.Request{
		Buf:        view,
		BufID:      b.hlID,
		Path:       b.Path,
		FileType:   h.filetype,
		Text:       b.Text(),
		First:      0,
		Last:       b.LinesNum() - 1,
		Generation: h.requestGen,
		Cause:      h.cause,
		Edits:      append([]syntax.Edit(nil), h.edits...),
	}
	h.edits = h.edits[:0]
	h.cause = syntax.CauseEdit
	req.Reply = func(spans []syntax.Span) {
		b.ApplySpans(req.Generation, spans)
	}
	req.Fail = func(error) {
		if b.hl != h || h.closed || h.requestGen != req.Generation {
			return
		}
		h.inFlight = false
		if b.hlGen != req.Generation {
			if h.dirty {
				b.scheduleHighlight()
			}
			return
		}
		h.dirty = false
	}

	p := h.provider
	defer func() {
		if r := recover(); r != nil {
			log.Println("syntax provider", p.Name, "panic:", r)
			b.providerPanicked(h)
		}
	}()
	p.Request(req)
}

func (b *SharedBuffer) providerPanicked(h *highlightSession) {
	if b.hl != h || h.closed {
		return
	}
	h.inFlight = false
	h.dirty = false
	if syntax.ReportError(h.provider.Name) {
		log.Println("syntax: disabling provider", h.provider.Name)
		b.detachHighlight(true)
	}
}

// ApplySpans replaces all highlighting with spans from generation gen.
// Lines not mentioned in spans are cleared. Stale generations are dropped.
func (b *SharedBuffer) ApplySpans(gen uint64, spans []syntax.Span) bool {
	h := b.hl
	if h == nil || h.closed {
		return false
	}
	if gen != b.hlGen {
		if gen == h.requestGen {
			h.inFlight = false
			if h.dirty {
				b.scheduleHighlight()
			}
		}
		return false
	}

	h.inFlight = false
	h.dirty = false
	syntax.ReportSuccess(h.provider.Name)

	byLine := make(map[int][]syntax.Span)
	for _, s := range spans {
		if s.Line < 0 || s.Line >= b.LinesNum() || s.EndByte <= s.StartByte {
			continue
		}
		byLine[s.Line] = append(byLine[s.Line], s)
	}

	for i := 0; i < b.LinesNum(); i++ {
		if lineSpans, ok := byLine[i]; ok {
			m := syntax.BuildMatch(b.LineBytes(i), lineSpans)
			if m == nil {
				m = plainMatch
			}
			b.SetMatch(i, m)
		} else {
			// An explicit index-0 entry stops the previous line's style
			// from bleeding into an unstyled line.
			b.SetMatch(i, plainMatch)
		}
	}

	screen.Redraw()
	return true
}

// InvalidateSyntax asks for fresh spans with a manual cause.
func (b *Buffer) InvalidateSyntax() {
	s := b.SharedBuffer
	if s == nil {
		return
	}
	if s.hl == nil || s.hl.closed {
		s.updateProvider()
		return
	}
	s.hlGen++
	s.hl.cause = syntax.CauseManual
	s.scheduleHighlight()
}

// RefreshSyntax re-resolves highlighting for every open buffer, usually
// after plugins register or unregister providers.
func RefreshSyntax() {
	seen := make(map[*SharedBuffer]bool)
	for _, b := range OpenBuffers {
		if b.SharedBuffer == nil || seen[b.SharedBuffer] {
			continue
		}
		seen[b.SharedBuffer] = true
		b.UpdateProvider()
	}
	screen.Redraw()
}

// ClearMatches clears all of the syntax highlighting for the buffer.
func (b *SharedBuffer) ClearMatches() {
	for i := range b.lines {
		b.SetMatch(i, nil)
	}
}

func (b *SharedBuffer) closeHighlight() {
	b.detachHighlight(true)
}

// IndentString returns this buffer's indent method (a tabstop or n spaces
// depending on the settings)
func (b *Buffer) IndentString(tabsize int) string {
	if b.Settings["tabstospaces"].(bool) {
		return util.Spaces(tabsize)
	}
	return "\t"
}

// SetCursors resets this buffer's cursors to a new list
func (b *Buffer) SetCursors(c []*Cursor) {
	b.cursors = c
	b.EventHandler.cursors = b.cursors
	b.EventHandler.active = b.curCursor
}

// AddCursor adds a new cursor to the list
func (b *Buffer) AddCursor(c *Cursor) {
	b.cursors = append(b.cursors, c)
	b.EventHandler.cursors = b.cursors
	b.EventHandler.active = b.curCursor
	b.UpdateCursors()
}

// SetCurCursor sets the current cursor
func (b *Buffer) SetCurCursor(n int) {
	b.curCursor = n
}

// GetActiveCursor returns the main cursor in this buffer
func (b *Buffer) GetActiveCursor() *Cursor {
	return b.cursors[b.curCursor]
}

// GetCursor returns the nth cursor
func (b *Buffer) GetCursor(n int) *Cursor {
	return b.cursors[n]
}

// GetCursors returns the list of cursors in this buffer
func (b *Buffer) GetCursors() []*Cursor {
	return b.cursors
}

// NumCursors returns the number of cursors
func (b *Buffer) NumCursors() int {
	return len(b.cursors)
}

// MergeCursors merges any cursors that are at the same position
// into one cursor
func (b *Buffer) MergeCursors() {
	var cursors []*Cursor
	for i := 0; i < len(b.cursors); i++ {
		c1 := b.cursors[i]
		if c1 != nil {
			for j := 0; j < len(b.cursors); j++ {
				c2 := b.cursors[j]
				if c2 != nil && i != j && c1.Loc == c2.Loc {
					b.cursors[j] = nil
				}
			}
			cursors = append(cursors, c1)
		}
	}

	b.cursors = cursors

	for i := range b.cursors {
		b.cursors[i].Num = i
	}

	if b.curCursor >= len(b.cursors) {
		b.curCursor = len(b.cursors) - 1
	}
	b.EventHandler.cursors = b.cursors
	b.EventHandler.active = b.curCursor
}

// UpdateCursors updates all the cursors indices
func (b *Buffer) UpdateCursors() {
	b.EventHandler.cursors = b.cursors
	b.EventHandler.active = b.curCursor
	for i, c := range b.cursors {
		c.Num = i
	}
}

func (b *Buffer) RemoveCursor(i int) {
	copy(b.cursors[i:], b.cursors[i+1:])
	b.cursors[len(b.cursors)-1] = nil
	b.cursors = b.cursors[:len(b.cursors)-1]
	b.curCursor = util.Clamp(b.curCursor, 0, len(b.cursors)-1)
	b.UpdateCursors()
}

// ClearCursors removes all extra cursors
func (b *Buffer) ClearCursors() {
	for i := 1; i < len(b.cursors); i++ {
		b.cursors[i] = nil
	}
	b.cursors = b.cursors[:1]
	b.UpdateCursors()
	b.curCursor = 0
	b.GetActiveCursor().Deselect(true)
}

// MoveLinesUp moves the range of lines up one row
func (b *Buffer) MoveLinesUp(start int, end int) {
	if start < 1 || start >= end || end > len(b.lines) {
		return
	}
	l := string(b.LineBytes(start - 1))
	if end == len(b.lines) {
		b.insert(
			Loc{
				util.CharacterCount(b.lines[end-1].data),
				end - 1,
			},
			[]byte{'\n'},
		)
	}
	b.Insert(
		Loc{0, end},
		l+"\n",
	)
	b.Remove(
		Loc{0, start - 1},
		Loc{0, start},
	)
}

// MoveLinesDown moves the range of lines down one row
func (b *Buffer) MoveLinesDown(start int, end int) {
	if start < 0 || start >= end || end >= len(b.lines) {
		return
	}
	l := string(b.LineBytes(end))
	b.Insert(
		Loc{0, start},
		l+"\n",
	)
	end++
	b.Remove(
		Loc{0, end},
		Loc{0, end + 1},
	)
}

var BracePairs = [][2]rune{
	{'(', ')'},
	{'{', '}'},
	{'[', ']'},
}

func (b *Buffer) findMatchingBrace(braceType [2]rune, start Loc, char rune) (Loc, bool) {
	var i int
	if char == braceType[0] {
		for y := start.Y; y < b.LinesNum(); y++ {
			l := []rune(string(b.LineBytes(y)))
			xInit := 0
			if y == start.Y {
				xInit = start.X
			}
			for x := xInit; x < len(l); x++ {
				r := l[x]
				if r == braceType[0] {
					i++
				} else if r == braceType[1] {
					i--
					if i == 0 {
						return Loc{x, y}, true
					}
				}
			}
		}
	} else if char == braceType[1] {
		for y := start.Y; y >= 0; y-- {
			l := []rune(string(b.lines[y].data))
			xInit := len(l) - 1
			if y == start.Y {
				xInit = start.X
			}
			for x := xInit; x >= 0; x-- {
				r := l[x]
				if r == braceType[1] {
					i++
				} else if r == braceType[0] {
					i--
					if i == 0 {
						return Loc{x, y}, true
					}
				}
			}
		}
	}
	return start, false
}

// If there is a brace character (for example '{' or ']') at the given start location,
// FindMatchingBrace returns the location of the matching brace for it (for example '}'
// or '['). The second returned value is true if there was no matching brace found
// for given starting location but it was found for the location one character left
// of it. The third returned value is true if the matching brace was found at all.
func (b *Buffer) FindMatchingBrace(start Loc) (Loc, bool, bool) {
	// TODO: maybe can be more efficient with utf8 package
	curLine := []rune(string(b.LineBytes(start.Y)))

	// first try to find matching brace for the given location (it has higher priority)
	if start.X >= 0 && start.X < len(curLine) {
		startChar := curLine[start.X]

		for _, bp := range BracePairs {
			if startChar == bp[0] || startChar == bp[1] {
				mb, found := b.findMatchingBrace(bp, start, startChar)
				if found {
					return mb, false, true
				}
			}
		}
	}

	if b.Settings["matchbraceleft"].(bool) {
		// failed to find matching brace for the given location, so try to find matching
		// brace for the location one character left of it
		if start.X-1 >= 0 && start.X-1 < len(curLine) {
			leftChar := curLine[start.X-1]
			left := Loc{start.X - 1, start.Y}

			for _, bp := range BracePairs {
				if leftChar == bp[0] || leftChar == bp[1] {
					mb, found := b.findMatchingBrace(bp, left, leftChar)
					if found {
						return mb, true, true
					}
				}
			}
		}
	}

	return start, false, false
}

// Retab changes all tabs to spaces or vice versa
func (b *Buffer) Retab() {
	toSpaces := b.Settings["tabstospaces"].(bool)
	tabsize := util.IntOpt(b.Settings["tabsize"])

	for i := 0; i < b.LinesNum(); i++ {
		l := b.LineBytes(i)

		ws := util.GetLeadingWhitespace(l)
		if len(ws) != 0 {
			if toSpaces {
				ws = bytes.ReplaceAll(ws, []byte{'\t'}, bytes.Repeat([]byte{' '}, tabsize))
			} else {
				ws = bytes.ReplaceAll(ws, bytes.Repeat([]byte{' '}, tabsize), []byte{'\t'})
			}
		}

		l = bytes.TrimLeft(l, " \t")

		b.Lock()
		b.lines[i].data = append(ws, l...)
		b.Unlock()

		b.MarkModified(i, i)
	}

	b.setModified()
}

// ParseCursorLocation turns a cursor location like 10:5 (LINE:COL)
// into a loc
func ParseCursorLocation(cursorPositions []string) (Loc, error) {
	startpos := Loc{0, 0}
	var err error

	// if no positions are available exit early
	if cursorPositions == nil {
		return startpos, errors.New("No cursor positions were provided.")
	}

	startpos.Y, err = strconv.Atoi(cursorPositions[0])
	startpos.Y--
	if err == nil {
		if len(cursorPositions) > 1 {
			startpos.X, err = strconv.Atoi(cursorPositions[1])
			if startpos.X > 0 {
				startpos.X--
			}
		}
	}

	return startpos, err
}

// Line returns the string representation of the given line number
func (b *Buffer) Line(i int) string {
	return string(b.LineBytes(i))
}

func (b *Buffer) Write(bytes []byte) (n int, err error) {
	b.EventHandler.InsertBytes(b.End(), bytes)
	return len(bytes), nil
}

func (b *Buffer) updateDiff(synchronous bool) {
	b.diffLock.Lock()
	defer b.diffLock.Unlock()

	b.diff = make(map[int]DiffStatus)

	if b.diffBase == nil {
		return
	}

	differ := dmp.New()

	if !synchronous {
		b.Lock()
	}
	bytes := b.Bytes()
	if !synchronous {
		b.Unlock()
	}

	baseRunes, bufferRunes, _ := differ.DiffLinesToRunes(string(b.diffBase), string(bytes))
	diffs := differ.DiffMainRunes(baseRunes, bufferRunes, false)
	lineN := 0

	for _, diff := range diffs {
		lineCount := len([]rune(diff.Text))

		switch diff.Type {
		case dmp.DiffEqual:
			lineN += lineCount
		case dmp.DiffInsert:
			var status DiffStatus
			if b.diff[lineN] == DSDeletedAbove {
				status = DSModified
			} else {
				status = DSAdded
			}
			for i := 0; i < lineCount; i++ {
				b.diff[lineN] = status
				lineN++
			}
		case dmp.DiffDelete:
			b.diff[lineN] = DSDeletedAbove
		}
	}
}

// UpdateDiff computes the diff between the diff base and the buffer content.
// The update may be performed synchronously or asynchronously.
// If an asynchronous update is already pending when UpdateDiff is called,
// UpdateDiff does not schedule another update.
func (b *Buffer) UpdateDiff() {
	if b.updateDiffTimer != nil {
		return
	}

	lineCount := b.LinesNum()
	if b.diffBaseLineCount > lineCount {
		lineCount = b.diffBaseLineCount
	}

	if lineCount < 1000 {
		b.updateDiff(true)
	} else if lineCount < 30000 {
		b.updateDiffTimer = time.AfterFunc(500*time.Millisecond, func() {
			b.updateDiffTimer = nil
			b.updateDiff(false)
			screen.Redraw()
		})
	} else {
		// Don't compute diffs for very large files
		b.diffLock.Lock()
		b.diff = make(map[int]DiffStatus)
		b.diffLock.Unlock()
	}
}

// SetDiffBase sets the text that is used as the base for diffing the buffer content
func (b *Buffer) SetDiffBase(diffBase []byte) {
	b.diffBase = diffBase
	if diffBase == nil {
		b.diffBaseLineCount = 0
	} else {
		b.diffBaseLineCount = strings.Count(string(diffBase), "\n")
	}
	b.UpdateDiff()
}

// DiffStatus returns the diff status for a line in the buffer
func (b *Buffer) DiffStatus(lineN int) DiffStatus {
	b.diffLock.RLock()
	defer b.diffLock.RUnlock()
	// Note that the zero value for DiffStatus is equal to DSUnchanged
	return b.diff[lineN]
}

// FindNextDiffLine returns the line number of the next block of diffs.
// If `startLine` is already in a block of diffs, lines in that block are skipped.
func (b *Buffer) FindNextDiffLine(startLine int, forward bool) (int, error) {
	if b.diff == nil {
		return 0, errors.New("no diff data")
	}
	startStatus, ok := b.diff[startLine]
	if !ok {
		startStatus = DSUnchanged
	}
	curLine := startLine
	for {
		curStatus, ok := b.diff[curLine]
		if !ok {
			curStatus = DSUnchanged
		}
		if curLine < 0 || curLine > b.LinesNum() {
			return 0, errors.New("no next diff hunk")
		}
		if curStatus != startStatus {
			if startStatus != DSUnchanged && curStatus == DSUnchanged {
				// Skip over the block of unchanged text
				startStatus = DSUnchanged
			} else {
				return curLine, nil
			}
		}
		if forward {
			curLine++
		} else {
			curLine--
		}
	}
}

// SearchMatch returns true if the given location is within a match of the last search.
// It is used for search highlighting
func (b *Buffer) SearchMatch(pos Loc) bool {
	return b.LineArray.SearchMatch(b, pos)
}

// WriteLog writes a string to the log buffer
func WriteLog(s string) {
	LogBuf.EventHandler.Insert(LogBuf.End(), s)
}

// GetLogBuf returns the log buffer
func GetLogBuf() *Buffer {
	return LogBuf
}
