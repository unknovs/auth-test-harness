package identities

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test people's codes are assembled at run time rather than written as literals:
// an identifier-shaped constant in a published repository cannot be told from a
// real person's code. Each person is one repeated digit.
func testCode(country, digit string) string {
	return "PNO" + country + "-" + strings.Repeat(digit, 6) + "-" + strings.Repeat(digit, 5)
}

func entry(serial, given, family string) string {
	return fmt.Sprintf(`{"serial_number": %q, "given_name": %q, "family_name": %q}`, serial, given, family)
}

func listJSON(entries ...string) string {
	return `{"identities": [` + strings.Join(entries, ",") + `]}`
}

// logs collects what the list logs, safely from any goroutine.
type logs struct {
	mu    sync.Mutex
	lines []string
}

func (l *logs) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *logs) count(substr string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, line := range l.lines {
		if strings.Contains(line, substr) {
			n++
		}
	}

	return n
}

// write puts content at path and moves its modification time forward, so a
// rewrite within the same clock tick still reads as a change.
func write(t *testing.T, path, content string, tick int) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(time.Duration(tick) * time.Minute)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestParseAcceptsAListAndLooksUpByEitherForm(t *testing.T) {
	a, b := testCode("LV", "1"), testCode("LV", "2")
	ix, err := parse([]byte(listJSON(entry(a, "Anna", "Bērziņa"), entry(" "+b+" ", " Jānis ", "Ozols"))))
	if err != nil {
		t.Fatal(err)
	}

	for _, typed := range []string{a, strings.ToLower(a), strings.TrimPrefix(a, "PNOLV-"), "  " + a + "  "} {
		p, ok := ix.lookup(typed)
		if !ok || p.GivenName != "Anna" || p.FamilyName != "Bērziņa" || p.SerialNumber != a {
			t.Errorf("lookup(%q) = %+v, %v", typed, p, ok)
		}
	}
	// Surrounding spaces in the file are not part of the person.
	if p, ok := ix.lookup(b); !ok || p.SerialNumber != b || p.GivenName != "Jānis" {
		t.Errorf("lookup(b) = %+v, %v", p, ok)
	}
	if _, ok := ix.lookup(testCode("LV", "3")); ok {
		t.Error("a code that is not in the list was found")
	}
	if _, ok := ix.lookup(""); ok {
		t.Error("an empty code was found")
	}
}

// Two people whose codes differ only in the prefix are told apart by the full
// serial number alone; the code without its prefix names neither.
func TestParseSharedCodeUnderTwoPrefixes(t *testing.T) {
	lv, ee := testCode("LV", "4"), testCode("EE", "4")
	ix, err := parse([]byte(listJSON(entry(lv, "Anna", "A"), entry(ee, "Eva", "E"))))
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := ix.lookup(lv); p.GivenName != "Anna" {
		t.Errorf("lookup(lv) = %+v", p)
	}
	if p, _ := ix.lookup(ee); p.GivenName != "Eva" {
		t.Errorf("lookup(ee) = %+v", p)
	}
	if p, ok := ix.lookup(strings.TrimPrefix(lv, "PNOLV-")); ok {
		t.Errorf("the shared code without a prefix named %+v", p)
	}
}

func TestParseRefusesAnUnusableFile(t *testing.T) {
	a := testCode("LV", "1")
	cases := map[string]struct{ content, says string }{
		"not JSON":            {`identities: []`, "not a valid list"},
		"no identities":       {`{"people": []}`, "not a valid list"},
		"identities missing":  {`{}`, `no "identities"`},
		"a misspelt field":    {`{"identities": [{"serial_number": "` + a + `", "given_name": "A", "familyname": "B"}]}`, "familyname"},
		"no serial number":    {listJSON(entry("", "A", "B")), "entry 1 has no serial_number"},
		"no given name":       {listJSON(entry(a, "", "B")), "has no given_name"},
		"no family name":      {listJSON(entry(a, "A", " ")), "has no family_name"},
		"a repeated code":     {listJSON(entry(a, "A", "B"), entry(strings.ToLower(a), "C", "D")), "entry 2 repeats the serial_number of entry 1"},
		"content after it":    {listJSON(entry(a, "A", "B")) + `{}`, "content after the list"},
		"a truncated file":    {listJSON(entry(a, "A", "B"))[:30], "not a valid list"},
		"a field not yet had": {`{"identities": [{"serial_number": "` + a + `", "given_name": "A", "family_name": "B", "outcome": "x"}]}`, "outcome"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parse([]byte(tc.content))
			if err == nil || !strings.Contains(err.Error(), tc.says) {
				t.Fatalf("err = %v, want one saying %q", err, tc.says)
			}
		})
	}

	if _, err := parse([]byte(`{"identities": []}`)); err != nil {
		t.Errorf("an empty list is a list: %v", err)
	}
}

// A changed file is read again at the next lookup — no restart.
func TestListRereadsTheFileWhenItChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identities.json")
	a, b := testCode("LV", "1"), testCode("LV", "2")
	write(t, path, listJSON(entry(a, "Anna", "A")), 0)

	var log logs
	l := Open(path, log.logf)
	if _, ok := l.Lookup(a); !ok {
		t.Fatal("the first list was not loaded")
	}

	write(t, path, listJSON(entry(b, "Jānis", "B")), 1)
	if _, ok := l.Lookup(b); !ok {
		t.Fatal("a person added to the file is not found")
	}
	if _, ok := l.Lookup(a); ok {
		t.Fatal("a person removed from the file is still found")
	}
	if n := log.count("loaded: 1 person"); n != 2 {
		t.Errorf("logged %d loads, want 2: %v", n, log.lines)
	}

	// An unchanged file is not read again.
	l.Lookup(b)
	if n := log.count("loaded"); n != 2 {
		t.Errorf("an unchanged file was read again: %v", log.lines)
	}
}

// A file that cannot be used is refused with a log line, and the last good list
// stays in use until the file is fixed.
func TestListKeepsTheLastGoodListWhenTheFileIsBroken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identities.json")
	a, b := testCode("LV", "1"), testCode("LV", "2")
	write(t, path, listJSON(entry(a, "Anna", "A")), 0)

	var log logs
	l := Open(path, log.logf)

	write(t, path, `{"identities": [`, 1)
	if _, ok := l.Lookup(a); !ok {
		t.Fatal("a broken file took the last good list away")
	}
	if log.count("refused") != 1 || log.count("the last good list (1 person) stays in use") != 1 {
		t.Errorf("the refusal was not logged as such: %v", log.lines)
	}

	write(t, path, listJSON(entry(a, "Anna", "A"), entry(a, "Anna", "A")), 2)
	if _, ok := l.Lookup(a); !ok || l.Len() != 1 {
		t.Fatal("a file with a repeated code replaced the last good list")
	}

	write(t, path, listJSON(entry(b, "Jānis", "B")), 3)
	if _, ok := l.Lookup(b); !ok {
		t.Fatal("the fixed file was not loaded")
	}

	// A removed file keeps the list in use too; the cause is logged once.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, ok := l.Lookup(b); !ok {
			t.Fatal("a removed file took the last good list away")
		}
	}
	if n := log.count("cannot be read"); n != 1 {
		t.Errorf("a missing file was logged %d times, want once: %v", n, log.lines)
	}
}

// A file missing at start is not fatal: no one is found until it appears, and
// then it is used without a restart.
func TestListMissingAtStartIsUsedOnceItAppears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identities.json")
	a := testCode("LV", "1")

	var log logs
	l := Open(path, log.logf)
	if _, ok := l.Lookup(a); ok || l.Len() != 0 {
		t.Fatal("a missing file named someone")
	}
	if log.count("no one can sign in from the list until it is fixed") != 1 {
		t.Errorf("the missing file was not logged as such: %v", log.lines)
	}

	write(t, path, listJSON(entry(a, "Anna", "A")), 0)
	if _, ok := l.Lookup(a); !ok {
		t.Fatal("the file that appeared was not used")
	}
	if l.Path() != path {
		t.Errorf("Path() = %q", l.Path())
	}
}

// A broken file at start is refused the same way.
func TestListBrokenAtStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identities.json")
	write(t, path, `not json`, 0)

	var log logs
	l := Open(path, log.logf)
	if l.Len() != 0 || log.count("refused") != 1 || log.count("until it is fixed") != 1 {
		t.Fatalf("len %d, logs %v", l.Len(), log.lines)
	}
}

// Lookups from many logins at once, while the file changes, must not race (run
// with -race).
func TestListConcurrentLookups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identities.json")
	a := testCode("LV", "1")
	write(t, path, listJSON(entry(a, "Anna", "A")), 0)
	l := Open(path, (&logs{}).logf)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := l.Lookup(a); !ok {
				t.Error("a listed person was not found")
			}
		}()
	}
	write(t, path, listJSON(entry(a, "Anna", "A")), 1)
	wg.Wait()
}

// The example list the compose file mounts is a usable list: twenty made-up
// people, one code range, each found by its code.
func TestTheExampleListIsUsable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "examples", "identities", "identities.json"))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := parse(data)
	if err != nil {
		t.Fatalf("the example list is refused: %v", err)
	}
	if n := len(ix.people); n < 20 {
		t.Fatalf("the example list holds %d people, want at least 20", n)
	}
	for i := 1; i <= 20; i++ {
		code := fmt.Sprintf("PNOLV-000123-%05d", i)
		if p, ok := ix.lookup(code); !ok || p.GivenName == "" || p.FamilyName == "" {
			t.Errorf("%s: %+v, %v", code, p, ok)
		}
	}
}

// A built-in list is read once from its data and never looks at a file; data
// that is not a usable list is refused.
func TestBuiltin(t *testing.T) {
	if _, err := Builtin([]byte(`{"identities": [`)); err == nil {
		t.Fatal("a broken built-in list was accepted")
	}
	a := testCode("LV", "1")
	l, err := Builtin([]byte(listJSON(entry(a, "Anna", "A"))))
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := l.Lookup(a); !ok || p.GivenName != "Anna" || l.Len() != 1 || l.Path() != "" {
		t.Fatalf("lookup %+v %v, len %d, path %q", p, ok, l.Len(), l.Path())
	}
}
