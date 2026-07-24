package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestReadFileHappyAndPagination(t *testing.T) {
	root := t.TempDir()
	body := strings.Repeat("αβγ", 100) // multi-byte runes
	mustWrite(t, filepath.Join(root, "doc.md"), body)
	fs, err := newDocsFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}

	out, err := fs.readFile(map[string]any{"path": "doc.md", "max_chars": 50, "offset": 0})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("%+v", out)
	}
	data := out["data"].(map[string]any)
	content, _ := data["content"].(string)
	if utf8.RuneCountInString(content) != 50 {
		t.Fatalf("content runes=%d", utf8.RuneCountInString(content))
	}
	if data["has_more"] != true || data["truncated"] != true {
		t.Fatalf("meta=%+v", data)
	}
	next, _ := data["next_offset"].(int)
	if next != 50 {
		t.Fatalf("next_offset=%v", data["next_offset"])
	}

	page2, err := fs.readFile(map[string]any{"path": "doc.md", "max_chars": 10000, "offset": next})
	if err != nil {
		t.Fatal(err)
	}
	d2 := page2["data"].(map[string]any)
	if d2["has_more"] != false {
		t.Fatalf("page2=%+v", d2)
	}
	if d2["next_offset"] != nil {
		t.Fatalf("next_offset=%v want nil", d2["next_offset"])
	}
}

func TestReadFileEdges(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "ok.md"), "# hi\n")
	mustWrite(t, filepath.Join(root, "note.txt"), "not markdown\n")
	sub := filepath.Join(root, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	fs, err := newDocsFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		args    map[string]any
		code    string
		wantErr bool
	}{
		{"missing", map[string]any{"path": "nope.md"}, "", true}, // resolve treats missing as outside
		{"directory", map[string]any{"path": "subdir"}, "", true},
		{"txt", map[string]any{"path": "note.txt"}, "file_type_not_allowed", false},
		{"absolute", map[string]any{"path": "/etc/passwd"}, "", true},
		{"dotdot", map[string]any{"path": "../etc/passwd"}, "", true},
		{"nullbyte", map[string]any{"path": "ok\x00.md"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := fs.readFile(tc.args)
			if tc.wantErr {
				if err == nil && (out == nil || out["ok"] == true) {
					t.Fatalf("expected rejection, out=%+v", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if ok, _ := out["ok"].(bool); ok {
				t.Fatalf("expected fail, got %+v", out)
			}
			if tc.code != "" {
				code := out["error"].(map[string]any)["code"]
				if code != tc.code {
					t.Fatalf("code=%v want %s", code, tc.code)
				}
			}
		})
	}

	// offset past EOF clamps
	out, err := fs.readFile(map[string]any{"path": "ok.md", "offset": 99999})
	if err != nil {
		t.Fatal(err)
	}
	data := out["data"].(map[string]any)
	if data["content"] != "" || data["has_more"] != false {
		t.Fatalf("%+v", data)
	}
}

func TestListDirPaginationAndNotDir(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		mustWrite(t, filepath.Join(root, "f"+string(rune('a'+i))+".md"), "x\n")
	}
	mustWrite(t, filepath.Join(root, "solo.md"), "y\n")
	fs, err := newDocsFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}

	out, err := fs.listDir(map[string]any{"path": "", "max_entries": 2, "offset": 0})
	if err != nil {
		t.Fatal(err)
	}
	data := out["data"].(map[string]any)
	entries := data["entries"].([]map[string]any)
	if len(entries) != 2 || data["has_more"] != true {
		t.Fatalf("%+v", data)
	}
	next := data["next_offset"].(int)

	out2, err := fs.listDir(map[string]any{"path": ".", "max_entries": 100, "offset": next})
	if err != nil {
		t.Fatal(err)
	}
	d2 := out2["data"].(map[string]any)
	if d2["has_more"] != false {
		t.Fatalf("%+v", d2)
	}

	// list a file path → not_directory
	bad, err := fs.listDir(map[string]any{"path": "solo.md"})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := bad["ok"].(bool); ok {
		t.Fatal("expected not_directory")
	}
	if bad["error"].(map[string]any)["code"] != "not_directory" {
		t.Fatalf("%+v", bad["error"])
	}

	// offset beyond total
	far, err := fs.listDir(map[string]any{"path": "", "offset": 9999, "max_entries": 1})
	if err != nil {
		t.Fatal(err)
	}
	fd := far["data"].(map[string]any)
	if len(fd["entries"].([]map[string]any)) != 0 {
		t.Fatalf("%+v", fd)
	}
}

func TestNewDocsFilesystemMissing(t *testing.T) {
	_, err := newDocsFilesystem(filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSymlinkEscapeBlocked(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.md"), "leak\n")
	mustWrite(t, filepath.Join(root, "safe.md"), "ok\n")
	link := filepath.Join(root, "escape.md")
	if err := os.Symlink(filepath.Join(outside, "secret.md"), link); err != nil {
		t.Skip("symlink not supported:", err)
	}
	fs, err := newDocsFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fs.readFile(map[string]any{"path": "escape.md"})
	if err == nil {
		t.Fatal("symlink outside sandbox must fail")
	}
	if !strings.Contains(err.Error(), "outside") {
		t.Fatalf("err=%v", err)
	}
}

func TestFormatLSListingNilDirInfoAndFloatSize(t *testing.T) {
	root := t.TempDir()
	listing := formatLSListing(root, nil, []map[string]any{
		{"name": "a.md", "type": "file", "mode": "", "size": float64(12), "mtime": "not-a-time"},
		{"name": "d", "type": "directory", "mode": "drwxr-xr-x", "size": 0},
		{"name": "b.md", "type": "file", "size": 3},
	}, 3)
	if !strings.Contains(listing, "total 3") || !strings.Contains(listing, "d/") {
		t.Fatalf("%q", listing)
	}
}

func TestOkMapCountVariants(t *testing.T) {
	m1 := okMap(map[string]any{"count": 7}, false)
	if m1["meta"].(map[string]any)["count"] != 7 {
		t.Fatalf("%+v", m1)
	}
	m2 := okMap(map[string]any{"matches": []map[string]any{{"a": 1}}}, true)
	if m2["meta"].(map[string]any)["count"] != 1 || m2["meta"].(map[string]any)["truncated"] != true {
		t.Fatalf("%+v", m2)
	}
}

func TestHealerCoercions(t *testing.T) {
	params := map[string]any{
		"properties": map[string]any{
			"n":    map[string]any{"type": "integer"},
			"f":    map[string]any{"type": "number"},
			"b":    map[string]any{"type": "boolean"},
			"z":    map[string]any{"type": "null"},
			"multi": map[string]any{"type": []any{"integer", "string"}},
			"bad":  map[string]any{"type": 123},
		},
	}
	out := healArgs(map[string]any{
		"n":     "42",
		"f":     "3.5",
		"b":     "yes",
		"z":     "null",
		"multi": "9",
		"keep":  "x",
		"bad":   "1",
	}, params)
	if out["n"] != 42 {
		t.Fatalf("n=%v", out["n"])
	}
	if out["f"] != 3.5 {
		t.Fatalf("f=%v", out["f"])
	}
	if out["b"] != true {
		t.Fatalf("b=%v", out["b"])
	}
	if out["z"] != nil {
		t.Fatalf("z=%v", out["z"])
	}
	if out["multi"] != 9 {
		t.Fatalf("multi=%v", out["multi"])
	}
	if out["keep"] != "x" {
		t.Fatalf("keep=%v", out["keep"])
	}

	if healArgs(map[string]any{"a": 1}, nil)["a"] != 1 {
		t.Fatal("nil params")
	}
	if healValue(true, map[string]any{"type": "boolean"}) != true {
		t.Fatal("non-string passthrough")
	}
	if !isIntString("+7") || isIntString("+") || isIntString("1.2") || isIntString("") {
		t.Fatal("isIntString")
	}
	if asString(float64(3)) != "3" || asString(float64(1.5)) == "" || asString(json.Number("9")) != "9" || asString(nil) != "" {
		t.Fatalf("asString edges")
	}
	if asString(struct{}{}) == "" {
		t.Fatal("asString default")
	}
	if asInt(int64(4), 0) != 4 || asInt(json.Number("5"), 0) != 5 || asInt("6", 0) != 6 || asInt("x", 9) != 9 {
		t.Fatal("asInt")
	}
	if asBool(true, false) != true || asBool("no", true) != false || asBool("maybe", true) != true {
		t.Fatal("asBool")
	}
	if clamp(-1, 0, 10) != 0 || clamp(99, 0, 10) != 10 || clamp(5, 0, 10) != 5 {
		t.Fatal("clamp")
	}
}

func TestLooksLikeBadPattern(t *testing.T) {
	if !looksLikeBadPattern("Invalid regex") || !looksLikeBadPattern("syntax error") {
		t.Fatal("expected true")
	}
	if looksLikeBadPattern("permission denied") {
		t.Fatal("expected false")
	}
	if (patternError{msg: "x"}).Error() != "x" {
		t.Fatal("Error()")
	}
}

func TestGrepQueryValidationAndTruncate(t *testing.T) {
	root := t.TempDir()
	longLine := strings.Repeat("m", 600)
	mustWrite(t, filepath.Join(root, "big.md"), "needle "+longLine+"\n")
	fs, err := newDocsFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	prev := findRipgrep
	findRipgrep = func(string) (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { findRipgrep = prev })

	empty, err := fs.grep(map[string]any{"query": "  ", "path": ""})
	if err != nil {
		t.Fatal(err)
	}
	if empty["error"].(map[string]any)["code"] != "validation" {
		t.Fatalf("%+v", empty)
	}
	tooLong := strings.Repeat("q", 201)
	long, err := fs.grep(map[string]any{"query": tooLong, "path": ""})
	if err != nil {
		t.Fatal(err)
	}
	if long["error"].(map[string]any)["message"] != "query too long" {
		t.Fatalf("%+v", long)
	}

	hit, err := fs.grep(map[string]any{"query": "needle", "path": "big.md", "max_results": 1, "case_sensitive": true})
	if err != nil {
		t.Fatal(err)
	}
	data := hit["data"].(map[string]any)
	matches := data["matches"].([]map[string]any)
	if len(matches) != 1 {
		t.Fatalf("%#v", matches)
	}
	text, _ := matches[0]["text"].(string)
	if utf8.RuneCountInString(text) > 500 {
		t.Fatalf("text not truncated: %d", utf8.RuneCountInString(text))
	}

	// max_results truncate across files
	mustWrite(t, filepath.Join(root, "a.md"), "zzz\n")
	mustWrite(t, filepath.Join(root, "b.md"), "zzz\n")
	trunc, err := fs.grep(map[string]any{"query": "zzz", "path": "", "max_results": 1})
	if err != nil {
		t.Fatal(err)
	}
	if trunc["meta"].(map[string]any)["truncated"] != true {
		t.Fatalf("%+v", trunc["meta"])
	}
}

func TestParseRipgrepJSONSkipsOutsideAndNonMatch(t *testing.T) {
	root := t.TempDir()
	fs, err := newDocsFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "in.md")
	outside := filepath.Join(t.TempDir(), "out.md")
	raw := strings.Join([]string{
		`{"type":"begin","data":{}}`,
		`not-json`,
		`{"type":"match","data":{"path":{"text":"` + outside + `"},"lines":{"text":"bad\n"},"line_number":1}}`,
		`{"type":"match","data":{"path":{"text":"` + inside + `"},"lines":{"text":"` + strings.Repeat("Z", 600) + `\n"},"line_number":2}}`,
		`{"type":"match","data":{"path":{"text":"` + inside + `"},"lines":{"text":"second\n"},"line_number":3}}`,
	}, "\n")
	matches, truncated := parseRipgrepJSON(fs, []byte(raw), 1)
	if !truncated || len(matches) != 1 {
		t.Fatalf("matches=%#v truncated=%v", matches, truncated)
	}
	if utf8.RuneCountInString(matches[0]["text"].(string)) > 500 {
		t.Fatal("line not truncated")
	}
}

func TestGrepRipgrepInvalidPatternAndNoMatch(t *testing.T) {
	rg, err := execLookPath("rg")
	if err != nil {
		t.Skip("rg not on PATH")
	}
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.md"), "hello\n")
	fs, err := newDocsFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	prev := findRipgrep
	findRipgrep = func(string) (string, error) { return rg, nil }
	t.Cleanup(func() { findRipgrep = prev })

	bad, err := fs.grep(map[string]any{"query": "(", "path": ""})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := bad["ok"].(bool); ok {
		t.Fatalf("want validation fail: %+v", bad)
	}

	empty, err := fs.grep(map[string]any{"query": "no_such_token_xyz", "path": ""})
	if err != nil {
		t.Fatal(err)
	}
	if empty["data"].(map[string]any)["engine"] != "ripgrep" {
		t.Fatalf("%+v", empty)
	}
	if empty["data"].(map[string]any)["count"] != 0 {
		t.Fatalf("%+v", empty)
	}
}

func TestRelativeToRootFallback(t *testing.T) {
	fs := &docsFilesystem{root: t.TempDir()}
	// path on different volume / unrelated → Rel may fail on some OS; still return slash path
	got := fs.relativeToRoot("/totally/unrelated/path.md")
	if got == "" {
		t.Fatal("empty")
	}
}
