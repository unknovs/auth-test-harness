// Package identities holds the people a login may answer as, read from a list
// file: a code entered at the sign-in step is looked up, and that person is who
// the login answers as.
//
// The service carries a built-in list (Builtin), for use with no file at all. A
// file, when one is given, replaces it, and is meant to be changed while the
// service runs. Its path is a setting,
// it is checked again at every lookup and re-read when it has changed, and a file
// that cannot be used is refused with a log line while the last good list stays
// in use — so an edit with a typo in it does not take every login down.
package identities

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Person is one entry of the list: the identity code the login reports as its
// serial_number, and the person's names.
type Person struct {
	SerialNumber string `json:"serial_number"`
	GivenName    string `json:"given_name"`
	FamilyName   string `json:"family_name"`
}

// index is one parsed list: every person by serial number, and by the code
// alone, without the "PNOLV-"-style prefix in front of it, which is how a person
// types their own code.
type index struct {
	people map[string]Person
	bare   map[string]string // code without the prefix → serial number; absent when two people share it
}

func (ix index) lookup(code string) (Person, bool) {
	c := normalise(code)
	if p, ok := ix.people[c]; ok {
		return p, true
	}
	if s, ok := ix.bare[c]; ok {
		return ix.people[s], true
	}

	return Person{}, false
}

func normalise(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// withoutPrefix returns the code after a semantics-identifier prefix — three
// letters for the kind of identifier, two for the country, and a hyphen, as in
// "PNOLV-" — or "" when the code carries none.
func withoutPrefix(serial string) string {
	if len(serial) <= 6 || serial[5] != '-' {
		return ""
	}
	for _, r := range serial[:5] {
		if r < 'A' || r > 'Z' {
			return ""
		}
	}

	return serial[6:]
}

// parse reads a list file's content. Every entry needs a serial number and both
// names; a serial number may appear once; a field the format does not know is
// refused rather than ignored, so a misspelt name is reported, not dropped.
func parse(data []byte) (index, error) {
	var f struct {
		Identities *[]Person `json:"identities"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return index{}, fmt.Errorf("not a valid list: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return index{}, errors.New("not a valid list: content after the list")
	}
	if f.Identities == nil {
		return index{}, errors.New(`not a valid list: no "identities" in the file`)
	}

	ix := index{people: map[string]Person{}, bare: map[string]string{}}
	first := map[string]int{}
	shared := map[string]bool{}
	for i, p := range *f.Identities {
		n := i + 1
		p.SerialNumber = strings.TrimSpace(p.SerialNumber)
		p.GivenName = strings.TrimSpace(p.GivenName)
		p.FamilyName = strings.TrimSpace(p.FamilyName)
		switch {
		case p.SerialNumber == "":
			return index{}, fmt.Errorf("entry %d has no serial_number", n)
		case p.GivenName == "":
			return index{}, fmt.Errorf("entry %d (%s) has no given_name", n, p.SerialNumber)
		case p.FamilyName == "":
			return index{}, fmt.Errorf("entry %d (%s) has no family_name", n, p.SerialNumber)
		}
		key := normalise(p.SerialNumber)
		if at, seen := first[key]; seen {
			return index{}, fmt.Errorf("entry %d repeats the serial_number of entry %d (%s)", n, at, p.SerialNumber)
		}
		first[key] = n
		ix.people[key] = p

		if b := withoutPrefix(key); b != "" {
			if _, taken := ix.bare[b]; taken || shared[b] {
				// Two people share the code under different prefixes: only the
				// full serial number tells them apart.
				delete(ix.bare, b)
				shared[b] = true
			} else {
				ix.bare[b] = key
			}
		}
	}

	return ix, nil
}

// List is the identity list file, kept current: each lookup first checks
// whether the file has changed, and re-reads it if so.
type List struct {
	path string
	logf func(format string, args ...any)

	mu      sync.Mutex
	current index
	good    bool // a list has been loaded at least once
	seen    bool // the file's state below has been read
	mod     time.Time
	size    int64
	lastErr string // the last unreadable-file error logged, so it is logged once
	fixed   bool   // a built-in list: read once, never re-read
}

// Builtin is a list read once from data and never re-read: the list the
// service carries, used when no file is given.
func Builtin(data []byte) (*List, error) {
	ix, err := parse(data)
	if err != nil {
		return nil, err
	}

	return &List{current: ix, good: true, fixed: true}, nil
}

// Open reads the list at path, logging through logf what it loaded or refused.
// A file that is missing or unusable at start is logged, and no one can sign in
// from the list until it is fixed — no restart needed.
func Open(path string, logf func(format string, args ...any)) *List {
	l := &List{path: path, logf: logf, current: index{people: map[string]Person{}, bare: map[string]string{}}}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refresh()

	return l
}

// Path is the file the list is read from; "" for a built-in list.
func (l *List) Path() string { return l.path }

// Lookup returns the person a code names: the full serial number, or the code
// without its prefix, in any letter case.
func (l *List) Lookup(code string) (Person, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refresh()

	return l.current.lookup(code)
}

// Len is the number of people in the list in use.
func (l *List) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.current.people)
}

// refresh re-reads the file when its modification time or size has changed.
// Callers hold the lock.
func (l *List) refresh() {
	if l.fixed {
		return
	}
	fi, err := os.Stat(l.path)
	if err != nil {
		if msg := err.Error(); msg != l.lastErr {
			l.lastErr = msg
			l.logf("identity list %s cannot be read: %v; %s", l.path, err, l.inUse())
		}

		return
	}
	l.lastErr = ""
	if l.seen && fi.ModTime().Equal(l.mod) && fi.Size() == l.size {
		return
	}

	data, err := os.ReadFile(l.path)
	if err != nil {
		l.logf("identity list %s cannot be read: %v; %s", l.path, err, l.inUse())

		return
	}
	l.seen, l.mod, l.size = true, fi.ModTime(), fi.Size()

	ix, err := parse(data)
	if err != nil {
		l.logf("identity list %s refused: %v; %s", l.path, err, l.inUse())

		return
	}
	l.current, l.good = ix, true
	l.logf("identity list %s loaded: %s", l.path, count(len(ix.people)))
}

func (l *List) inUse() string {
	if !l.good {
		return "no one can sign in from the list until it is fixed"
	}

	return fmt.Sprintf("the last good list (%s) stays in use", count(len(l.current.people)))
}

func count(n int) string {
	if n == 1 {
		return "1 person"
	}

	return fmt.Sprintf("%d people", n)
}
