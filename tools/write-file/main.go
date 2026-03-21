// main.go provides a JSON-driven file writer that atomically creates or
// overwrites files and returns a unified diff for overwrites.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Input struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type Output struct {
	OK      bool   `json:"ok"`
	Created bool   `json:"created,omitempty"`
	Diff    string `json:"diff,omitempty"`
	Summary string `json:"summary,omitempty"`
	Error   string `json:"error,omitempty"`
}

func main() {
	var in Input
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		writeError("invalid JSON input: " + err.Error())
		return
	}

	if in.FilePath == "" {
		writeError("file_path is required")
		return
	}

	// Read existing content if file exists.
	oldContent, err := os.ReadFile(in.FilePath)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		writeError("cannot read existing file: " + err.Error())
		return
	}

	// Create parent directories.
	dir := filepath.Dir(in.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError("cannot create directories: " + err.Error())
		return
	}

	// Write atomically: temp file then rename.
	tmp, err := os.CreateTemp(dir, ".write-file-*")
	if err != nil {
		writeError("cannot create temp file: " + err.Error())
		return
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(in.Content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		writeError("cannot write temp file: " + err.Error())
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		writeError("cannot close temp file: " + err.Error())
		return
	}

	// Preserve permissions if overwriting.
	if existed {
		if info, err := os.Stat(in.FilePath); err == nil {
			os.Chmod(tmpName, info.Mode())
		}
	}

	if err := os.Rename(tmpName, in.FilePath); err != nil {
		os.Remove(tmpName)
		writeError("cannot write file: " + err.Error())
		return
	}

	lines := countLines(in.Content)
	bytes := len(in.Content)

	out := Output{OK: true, Created: !existed}
	if existed {
		out.Summary = fmt.Sprintf("Overwrote %s (%d lines, %d bytes)", in.FilePath, lines, bytes)
		out.Diff = unifiedDiff(in.FilePath, string(oldContent), in.Content)
	} else {
		out.Summary = fmt.Sprintf("Created %s (%d lines, %d bytes)", in.FilePath, lines, bytes)
	}

	writeJSON(out)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func writeError(msg string) {
	writeJSON(Output{OK: false, Error: msg})
}

func writeJSON(out Output) {
	json.NewEncoder(os.Stdout).Encode(out)
}

// --- Unified diff generation (same as edit-file) ---

func unifiedDiff(path, a, b string) string {
	aLines := splitLines(a)
	bLines := splitLines(b)

	edits := myersDiff(aLines, bLines)
	hunks := buildHunks(edits, 3)
	if len(hunks) == 0 {
		return ""
	}

	display := path
	if strings.HasPrefix(display, "/") {
		display = display[1:]
	}

	var sb strings.Builder
	sb.WriteString("--- a/" + display + "\n")
	sb.WriteString("+++ b/" + display + "\n")

	for _, h := range hunks {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", h.aStart+1, h.aCount, h.bStart+1, h.bCount))
		for _, line := range h.lines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type editOp int

const (
	opEqual  editOp = iota
	opDelete
	opInsert
)

type edit struct {
	op   editOp
	line string
}

func myersDiff(a, b []string) []edit {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		edits := make([]edit, m)
		for i, l := range b {
			edits[i] = edit{opInsert, l}
		}
		return edits
	}
	if m == 0 {
		edits := make([]edit, n)
		for i, l := range a {
			edits[i] = edit{opDelete, l}
		}
		return edits
	}

	max := n + m
	off := max
	size := 2*max + 1
	v := make([]int, size)
	v[off+1] = 0

	trace := make([][]int, 0, max)

	for d := 0; d <= max; d++ {
		vc := make([]int, size)
		copy(vc, v)
		trace = append(trace, vc)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				return backtrack(trace, a, b, d, off)
			}
		}
	}
	return nil
}

func backtrack(trace [][]int, a, b []string, d, off int) []edit {
	edits := make([]edit, 0, len(a)+len(b))
	x, y := len(a), len(b)

	for dd := d; dd > 0; dd-- {
		v := trace[dd]
		k := x - y
		var prevK int
		if k == -dd || (k != dd && v[off+k-1] < v[off+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[off+prevK]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			x--
			y--
			edits = append(edits, edit{opEqual, a[x]})
		}
		if x == prevX {
			y--
			edits = append(edits, edit{opInsert, b[y]})
		} else {
			x--
			edits = append(edits, edit{opDelete, a[x]})
		}
	}
	for x > 0 && y > 0 {
		x--
		y--
		edits = append(edits, edit{opEqual, a[x]})
	}

	for i, j := 0, len(edits)-1; i < j; i, j = i+1, j-1 {
		edits[i], edits[j] = edits[j], edits[i]
	}
	return edits
}

type hunk struct {
	aStart, aCount int
	bStart, bCount int
	lines          []string
}

func buildHunks(edits []edit, context int) []hunk {
	if len(edits) == 0 {
		return nil
	}

	type region struct{ start, end int }
	var regions []region
	i := 0
	for i < len(edits) {
		if edits[i].op != opEqual {
			start := i
			for i < len(edits) && edits[i].op != opEqual {
				i++
			}
			regions = append(regions, region{start, i})
		} else {
			i++
		}
	}
	if len(regions) == 0 {
		return nil
	}

	var hunks []hunk
	for _, r := range regions {
		cStart := r.start - context
		if cStart < 0 {
			cStart = 0
		}
		cEnd := r.end + context
		if cEnd > len(edits) {
			cEnd = len(edits)
		}

		if len(hunks) > 0 {
			prev := &hunks[len(hunks)-1]
			prevEnd := prevHunkEditEnd(edits, prev)
			if cStart <= prevEnd {
				extendHunk(prev, edits, prevEnd, cEnd)
				continue
			}
		}

		h := newHunk(edits, cStart, cEnd)
		hunks = append(hunks, h)
	}
	return hunks
}

func newHunk(edits []edit, start, end int) hunk {
	var h hunk
	aLine, bLine := 0, 0
	for i := 0; i < start; i++ {
		switch edits[i].op {
		case opEqual:
			aLine++
			bLine++
		case opDelete:
			aLine++
		case opInsert:
			bLine++
		}
	}
	h.aStart = aLine
	h.bStart = bLine

	for i := start; i < end; i++ {
		switch edits[i].op {
		case opEqual:
			h.lines = append(h.lines, " "+edits[i].line)
			h.aCount++
			h.bCount++
		case opDelete:
			h.lines = append(h.lines, "-"+edits[i].line)
			h.aCount++
		case opInsert:
			h.lines = append(h.lines, "+"+edits[i].line)
			h.bCount++
		}
	}
	return h
}

func prevHunkEditEnd(edits []edit, h *hunk) int {
	aLine, bLine := 0, 0
	for i := 0; i < len(edits); i++ {
		if aLine >= h.aStart+h.aCount && bLine >= h.bStart+h.bCount {
			return i
		}
		switch edits[i].op {
		case opEqual:
			aLine++
			bLine++
		case opDelete:
			aLine++
		case opInsert:
			bLine++
		}
	}
	return len(edits)
}

func extendHunk(h *hunk, edits []edit, from, to int) {
	for i := from; i < to; i++ {
		switch edits[i].op {
		case opEqual:
			h.lines = append(h.lines, " "+edits[i].line)
			h.aCount++
			h.bCount++
		case opDelete:
			h.lines = append(h.lines, "-"+edits[i].line)
			h.aCount++
		case opInsert:
			h.lines = append(h.lines, "+"+edits[i].line)
			h.bCount++
		}
	}
}
