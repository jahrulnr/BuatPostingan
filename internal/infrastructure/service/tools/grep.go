package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const grepTimeout = 10 * time.Second

// findRipgrep locates the rg binary. Overridable in tests to force the Go fallback.
var findRipgrep = exec.LookPath

func (fs *docsFilesystem) grep(args map[string]any) (map[string]any, error) {
	query := strings.TrimSpace(asString(args["query"]))
	if query == "" {
		return failMap("validation", "query required"), nil
	}
	if utf8.RuneCountInString(query) > 200 {
		return failMap("validation", "query too long"), nil
	}
	relative, err := fs.relativePath(asString(args["path"]))
	if err != nil {
		return nil, err
	}
	target, err := fs.resolve(relative, true)
	if err != nil {
		return nil, err
	}
	maxResults := clamp(asInt(args["max_results"], 30), 1, 100)
	caseSensitive := asBool(args["case_sensitive"], false)

	st, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("path is outside docs root")
	}

	if rgPath, err := findRipgrep("rg"); err == nil {
		matches, truncated, runErr := fs.grepRipgrep(rgPath, query, target, st.IsDir(), maxResults, caseSensitive)
		if runErr == nil {
			return grepOK(query, matches, truncated, "ripgrep"), nil
		}
		if isInvalidPattern(runErr) {
			return failMap("validation", "invalid regex pattern"), nil
		}
		if strings.Contains(runErr.Error(), "timed out") {
			return failMap("tool_error", "grep timed out"), nil
		}
		// rg unavailable/broken mid-flight → Go fallback.
	}

	matches, truncated, runErr := fs.grepGo(query, target, st.IsDir(), maxResults, caseSensitive)
	if runErr != nil {
		if isInvalidPattern(runErr) {
			return failMap("validation", "invalid regex pattern"), nil
		}
		return failMap("tool_error", "grep walk failed"), nil
	}
	return grepOK(query, matches, truncated, "go"), nil
}

func grepOK(query string, matches []map[string]any, truncated bool, engine string) map[string]any {
	out := okMap(map[string]any{
		"query":   query,
		"matches": matches,
		"count":   len(matches),
		"engine":  engine,
	}, truncated)
	return out
}

type patternError struct{ msg string }

func (e patternError) Error() string { return e.msg }

func isInvalidPattern(err error) bool {
	_, ok := err.(patternError)
	return ok
}

func (fs *docsFilesystem) grepRipgrep(rgPath, query, target string, isDir bool, maxResults int, caseSensitive bool) ([]map[string]any, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), grepTimeout)
	defer cancel()

	args := []string{
		"--json",
		"--color=never",
		"--no-config",
		"--no-messages",
	}
	if !caseSensitive {
		args = append(args, "--ignore-case")
	}
	if isDir {
		args = append(args, "--glob", "*.md")
	}
	// -e keeps patterns that start with "-" from being parsed as flags.
	args = append(args, "-e", query, "--", target)

	cmd := exec.CommandContext(ctx, rgPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, false, fmt.Errorf("grep timed out")
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code := ee.ExitCode()
			// rg exit 1 = no matches (success for our tool).
			if code == 1 {
				return []map[string]any{}, false, nil
			}
			// exit 2 often means bad pattern / usage.
			msg := strings.TrimSpace(stderr.String())
			if code == 2 || looksLikeBadPattern(msg) {
				return nil, false, patternError{msg: msg}
			}
		}
		return nil, false, err
	}

	matches, truncated := parseRipgrepJSON(fs, stdout.Bytes(), maxResults)
	return matches, truncated, nil
}

func looksLikeBadPattern(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "regex") ||
		strings.Contains(lower, "syntax") ||
		strings.Contains(lower, "parse") ||
		strings.Contains(lower, "invalid")
}

func parseRipgrepJSON(fs *docsFilesystem, raw []byte, maxResults int) ([]map[string]any, bool) {
	matches := make([]map[string]any, 0)
	sc := bufio.NewScanner(bytes.NewReader(raw))
	// Long lines in docs are rare; raise buffer for safety.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		var ev struct {
			Type string `json:"type"`
			Data struct {
				Path struct {
					Text string `json:"text"`
				} `json:"path"`
				Lines struct {
					Text string `json:"text"`
				} `json:"lines"`
				LineNumber int `json:"line_number"`
			} `json:"data"`
		}
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil || ev.Type != "match" {
			continue
		}
		text := strings.TrimRight(ev.Data.Lines.Text, "\r\n")
		if utf8.RuneCountInString(text) > 500 {
			text = string([]rune(text)[:500])
		}
		abs := filepath.Clean(ev.Data.Path.Text)
		if !withinRoot(fs.root, abs) {
			// Defensive: never surface paths outside sandbox.
			continue
		}
		matches = append(matches, map[string]any{
			"path": fs.relativeToRoot(abs),
			"line": ev.Data.LineNumber,
			"text": text,
		})
		if len(matches) >= maxResults {
			return matches, true
		}
	}
	return matches, false
}

func (fs *docsFilesystem) grepGo(query, target string, isDir bool, maxResults int, caseSensitive bool) ([]map[string]any, bool, error) {
	pattern := query
	if !caseSensitive {
		pattern = "(?i)" + query
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, false, patternError{msg: err.Error()}
	}

	var files []string
	if isDir {
		files, err = fs.markdownFiles(target)
		if err != nil {
			return nil, false, err
		}
	} else {
		files = []string{target}
	}

	matches := make([]map[string]any, 0)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		for i, line := range lines {
			if !re.MatchString(line) {
				continue
			}
			text := line
			if utf8.RuneCountInString(text) > 500 {
				text = string([]rune(text)[:500])
			}
			matches = append(matches, map[string]any{
				"path": fs.relativeToRoot(file),
				"line": i + 1,
				"text": text,
			})
			if len(matches) >= maxResults {
				return matches, true, nil
			}
		}
	}
	return matches, false, nil
}
