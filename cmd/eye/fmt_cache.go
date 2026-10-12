package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	"eyelang/check"
)

const fmtCacheVersion = 2

type fmtResult struct {
	Version int
	Files   []string
	Inputs  []check.FormatInput
	Stdout  []byte
	Stderr  []byte
	Code    int
	Stats   fmtSavedStats
}

// fmtSavedStats is what `eye fmt --stats` reports of the run that saved a
// result, so a replay can report it without reading the files again.
type fmtSavedStats struct {
	Saved               time.Time
	Took                time.Duration
	Lines, Bytes        int
	Unformatted, Errors int
	Programs, Modules   int
}

func fmtResultPath(paths []string, syntaxOnly bool) string {
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	request := struct {
		Version int
		Binary  string
		Cwd     string
		Paths   []string
		Syntax  bool
	}{fmtCacheVersion, fileIdentity(exe), cwd, paths, syntaxOnly}
	data, _ := json.Marshal(request)
	sum := sha256.Sum256(data)
	return filepath.Join(cache, "eyelang", "fmt", hex.EncodeToString(sum[:])+".json")
}

func validFmtInputs(inputs []check.FormatInput) bool {
	for _, input := range inputs {
		if !input.Matches() {
			return false
		}
	}
	return true
}

// coversFiles reports whether inputs hold the contents of every file to
// check, so a saved result whose inputs were lost or truncated isn't used.
func coversFiles(inputs []check.FormatInput, files []string) bool {
	read := map[string]bool{}
	for _, input := range inputs {
		if input.Kind == "file" {
			read[input.Path] = true
		}
	}
	for _, file := range absPaths(files) {
		if !read[file] {
			return false
		}
	}
	return true
}

func loadFmtResult(path string, files []string) (fmtResult, bool) {
	var result fmtResult
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 32<<20 || json.Unmarshal(data, &result) != nil ||
		result.Version != fmtCacheVersion || result.Code < 0 || result.Code > exitErrors ||
		!slices.Equal(result.Files, files) || !coversFiles(result.Inputs, files) ||
		!validFmtInputs(result.Inputs) {
		return fmtResult{}, false
	}
	return result, true
}

// saveFmtResult saves a check's result and returns how many inputs it
// depends on.
func saveFmtResult(path string, files []string, session *check.FormatSession, stdout, stderr []byte, code int, stats fmtSavedStats) int {
	inputs, stable := session.Inputs()
	if path == "" || !stable || !coversFiles(inputs, files) || !validFmtInputs(inputs) {
		return len(inputs)
	}
	result := fmtResult{fmtCacheVersion, files, inputs, stdout, stderr, code, stats}
	data, err := json.Marshal(result)
	if err == nil && len(data) <= 32<<20 {
		// A missing or unwritable cache affects speed, never check results.
		_ = atomicFile(path, data)
	}
	return len(inputs)
}
