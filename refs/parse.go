package refs

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Parse
func Parse(ref string) (Ref, error) {
	_, items := lex("ref", ref)

	var outRef Ref

	var allItems []string
	for item := range items {
		allItems = append(allItems, item.String())
		switch {
		case item.typ == itemGlobal:
			outRef.Global = true
		case item.typ == itemRepo:
			outRef.Repo = item.val
		case item.typ == itemError:
			return Ref{}, fmt.Errorf("%s", item.val)
		case item.typ == itemPackage:
			outRef.Filename = item.val
		case item.typ == itemSubpathType:
			pt := SubPathType(item.val)
			outRef.SubPathType = pt
		case item.typ == itemSubpath:
			outRef.SubPath = strings.TrimSuffix(item.val, "/")
		case item.typ == itemRelease:
			outRef.ReleaseOrIntent = ReleaseOrIntent{
				Type:  Release,
				Value: item.val,
			}
		case item.typ == itemIntent:
			outRef.ReleaseOrIntent = ReleaseOrIntent{
				Type:  Intent,
				Value: item.val,
			}
		case item.typ == itemFragment:
			outRef.Fragment = strings.TrimSuffix(item.val, "/")
		}
	}

	if false {
		fmt.Println(ref, strings.Join(allItems, " "))
	}

	return outRef, outRef.Valid()
}

type itemType int

type item struct {
	typ itemType
	val string
}

const (
	itemRepo itemType = iota
	itemPackage

	itemGlobal

	itemRelease
	itemIntent

	itemSubpathType
	itemSubpath

	itemFragment

	itemError
	itemEOF
)

const (
	hash          = "#"
	slash         = "/"
	releasePrefix = "@"
	intentPrefix  = "+"
	repoSeparator = "-"
)

const eof = -1

func (i item) String() string {
	switch i.typ {
	case itemRepo:
		return fmt.Sprintf("itemRepo(%s)", i.val)
	case itemPackage:
		return fmt.Sprintf("itemPackage(%s)", i.val)
	case itemFragment:
		return fmt.Sprintf("itemWorkPath(%s)", i.val)
	case itemSubpathType:
		return fmt.Sprintf("itemTypeIdentifier(%s)", i.val)
	case itemSubpath:
		return fmt.Sprintf("itemName(%s)", i.val)
	case itemEOF:
		return "EOF"
	case itemRelease:
		return fmt.Sprintf("itemRelease(%s)", i.val)
	default:
		return fmt.Sprintf("itemType(%d)", i.typ)
	}
}

// lexer holds the state of the scanner.
type lexer struct {
	name  string    // used only for error reports.
	input string    // the string being scanned.
	start int       // start position of this item.
	pos   int       // current position in the input.
	width int       // width of last rune read from input.
	items chan item // channel of scanned items.
}

type stateFn func(*lexer) stateFn

func lex(name, input string) (*lexer, chan item) {
	l := &lexer{
		name:  name,
		input: input,
		items: make(chan item),
	}
	go l.run() // Concurrently run state machine.
	return l, l.items
}

// run lexes the input by executing state functions until
// the state is nil.
func (l *lexer) run() {
	for state := lexStart; state != nil; {
		state = state(l)
	}
	close(l.items) // No more tokens will be delivered.
}

func lexStart(l *lexer) stateFn {
	// Support // as an indicator for the root of the current repo
	if strings.HasPrefix(l.input[l.pos:], "//") {
		l.pos += len("//")
		l.ignore()
		l.items <- item{itemRepo, "."}
		return lexPath
	}

	if strings.HasPrefix(l.input[l.pos:], "./") {
		if strings.HasPrefix(l.input[l.pos:], "./+") || strings.HasPrefix(l.input[l.pos:], "./@") {
			l.pos += len(".")
			l.emit(itemPackage)
			if strings.HasPrefix(l.input[l.pos:], slash+intentPrefix) {
				l.pos += len(slash)
				l.ignore()
				return lexIntent
			}
			if strings.HasPrefix(l.input[l.pos:], slash+releasePrefix) {
				l.pos += len(slash)
				l.ignore()
				return lexRelease
			}
			return l.errorf("expected intent or release")
		}
		if !strings.HasPrefix(l.input[l.pos:], "./-") {
			l.pos += len(".")
			l.emit(itemPackage)
			l.pos += len("/")
			l.ignore()
			return lexSubpathType
		}
	}
	if strings.HasPrefix(l.input[l.pos:], releasePrefix) {
		l.emit(itemGlobal)
		return lexRelease
	}

	if strings.HasPrefix(l.input[l.pos:], intentPrefix) {
		l.emit(itemGlobal)
		return lexIntent
	}

	return lexPath
}

func lexPath(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], slash+repoSeparator) {
			l.emit(itemRepo)
			l.pos += len(slash + repoSeparator)
			l.ignore()

			n := l.next()
			if n == eof {
				l.emit(itemEOF)
				return nil
			} else if n == '/' {
				l.ignore()
				return lexPath
			} else {
				return l.errorf("expected slash or eof after repo")
			}
		}

		if strings.HasPrefix(l.input[l.pos:], slash+releasePrefix) {
			l.emit(itemPackage)
			_ = l.next()
			return lexRelease // Next state.
		}

		if strings.HasPrefix(l.input[l.pos:], slash+intentPrefix) {
			l.emit(itemPackage)
			_ = l.next()
			return lexIntent
		}

		if strings.HasPrefix(l.input[l.pos:], releasePrefix) {
			if l.pos > l.start {
				return l.errorf("unexpected release prefix in path")
			}
			return lexRelease
		}

		if strings.HasPrefix(l.input[l.pos:], intentPrefix) {
			if l.pos > l.start {
				return l.errorf("unexpected intent prefix in path")
			}
			return lexIntent
		}

		if l.next() == eof {
			break
		}
	}
	// Correctly reached EOF.
	l.emit(itemPackage)
	l.emit(itemEOF) // Useful to make EOF a token.
	return nil      // Stop the run loop.
}

func lexRelease(l *lexer) stateFn {
	l.pos += len(releasePrefix)
	l.ignore() // Ignore the prefixing
	for {
		if strings.HasPrefix(l.input[l.pos:], slash) {
			l.emit(itemRelease)
			l.pos += len(slash)
			l.ignore()
			return lexSubpathType
		}
		if strings.HasPrefix(l.input[l.pos:], hash) {
			l.emit(itemRelease)
			l.pos += len(hash)
			l.ignore()
			return lexFragment
		}
		if l.next() == eof {
			l.emit(itemRelease)
			l.emit(itemEOF)
			return nil
		}
	}
}

func lexIntent(l *lexer) stateFn {
	l.pos += len(intentPrefix)
	l.ignore() // Ignore the prefixing
	for {
		if strings.HasPrefix(l.input[l.pos:], slash) {
			l.emit(itemIntent)
			l.pos += len(slash)
			l.ignore()
			return lexSubpathType
		}
		if strings.HasPrefix(l.input[l.pos:], hash) {
			l.emit(itemIntent)
			l.pos += len(hash)
			l.ignore()
			return lexFragment
		}
		if l.next() == eof {
			l.emit(itemIntent)
			l.emit(itemEOF)
			return nil
		}
	}
}

func lexSubpathType(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], slash) {
			l.emit(itemSubpathType)

			l.pos += len(slash)
			l.ignore()
			return lexSubpath
		}
		if strings.HasPrefix(l.input[l.pos:], hash) {
			return l.errorf("unexpected fragment")
		}
		if l.next() == eof {
			l.backup()
			l.emit(itemSubpathType)
			return lexPath
		}
	}
}

func lexSubpath(l *lexer) stateFn {
	l.pos += len(slash)
	for {
		if strings.HasPrefix(l.input[l.pos:], slash+releasePrefix) {
			return l.errorf("unexpected release prefix in subpath")
		}
		if strings.HasPrefix(l.input[l.pos:], hash) {
			l.emit(itemSubpath)
			l.pos += len(hash)
			l.ignore()
			return lexFragment
		}
		n := l.next()
		if n == '=' {
			return l.errorf("unexpected '='")
		}
		if n == eof {
			l.emit(itemSubpath)
			l.emit(itemEOF)
			return nil
		}
	}
}

func lexFragment(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], slash+releasePrefix) {
			return l.errorf("unexpected release prefix in fragment")
		}
		if l.next() == eof {
			break
		}
	}
	// Correctly reached EOF.
	if l.pos > l.start {
		l.emit(itemFragment)
	}
	l.emit(itemEOF) // Useful to make EOF a token.
	return nil      // Stop the run loop.
}

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.input[l.start:l.pos]}
	l.start = l.pos
}

// next returns the next rune in the input.
func (l *lexer) next() (rune rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	rune, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return rune
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.start = l.pos
}

// backup steps back one rune.
// Can be called only once per call of next.
func (l *lexer) backup() {
	l.pos -= l.width
}

// peek returns but does not consume
// the next rune in the input.
func (l *lexer) peek() rune {
	rune := l.next()
	l.backup()
	return rune
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

// accept consumes the next rune
// if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

// error returns an error token and terminates the scan
// by passing back a nil pointer that will be the next
// state, terminating l.run.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{
		itemError,
		fmt.Sprintf(format, args...),
	}
	return nil
}
