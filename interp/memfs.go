package interp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"eyelang/ir"
)

// memFS is a file system held in memory, which the fs module reads and
// writes in place of the disk's (RunOptions.FS, TestOptions.MemoryFS): the
// online playground's files, where a browser has no disk. Paths are clean
// and absolute; a relative one is below dir.
type memFS struct {
	dir   string
	files map[string][]byte
	dirs  map[string]bool
}

// newMemFS holds files, by absolute path, and the directories they are in.
func newMemFS(dir string, files map[string][]byte) *memFS {
	m := &memFS{dir: filepath.Clean(dir), files: map[string][]byte{}, dirs: map[string]bool{}}
	m.addDirs(m.dir)
	for p, data := range files {
		p = filepath.Clean(p)
		m.files[p] = append([]byte(nil), data...)
		m.addDirs(filepath.Dir(p))
	}
	return m
}

func (m *memFS) addDirs(dir string) {
	for d := dir; !m.dirs[d]; d = filepath.Dir(d) {
		m.dirs[d] = true
		if filepath.Dir(d) == d {
			break
		}
	}
}

func notFound(op, path string) error {
	return &fs.PathError{Op: op, Path: path, Err: fs.ErrNotExist}
}

func (m *memFS) readFile(p string) ([]byte, error) {
	data, ok := m.files[p]
	if !ok {
		if m.dirs[p] {
			return nil, &fs.PathError{Op: "read", Path: p, Err: errors.New("is a directory")}
		}
		return nil, notFound("open", p)
	}
	return append([]byte(nil), data...), nil
}

// writeFile writes data to the file at p, after what it holds when
// appending. The file's directory must exist, as on a disk.
func (m *memFS) writeFile(p string, data []byte, appending bool) error {
	if !m.dirs[filepath.Dir(p)] {
		return notFound("open", p)
	}
	if m.dirs[p] {
		return &fs.PathError{Op: "open", Path: p, Err: errors.New("is a directory")}
	}
	if appending {
		data = append(append([]byte(nil), m.files[p]...), data...)
	}
	m.files[p] = append([]byte(nil), data...)
	return nil
}

func (m *memFS) exists(p string) bool {
	_, file := m.files[p]
	return file || m.dirs[p]
}

// list returns the names in the directory at p, sorted.
func (m *memFS) list(p string) ([]string, error) {
	if !m.dirs[p] {
		return nil, notFound("open", p)
	}
	var names []string
	add := func(child string) {
		if filepath.Dir(child) == p && child != p {
			names = append(names, filepath.Base(child))
		}
	}
	for f := range m.files {
		add(f)
	}
	for d := range m.dirs {
		add(d)
	}
	sort.Strings(names)
	return names, nil
}

func (m *memFS) makeDir(p string) error {
	if _, file := m.files[p]; file {
		return &fs.PathError{Op: "mkdir", Path: p, Err: fs.ErrExist}
	}
	m.addDirs(p)
	return nil
}

// remove removes a file, or a directory with nothing in it.
func (m *memFS) remove(p string) error {
	if _, file := m.files[p]; file {
		delete(m.files, p)
		return nil
	}
	if !m.dirs[p] {
		return notFound("remove", p)
	}
	prefix := p + string(filepath.Separator)
	for f := range m.files {
		if strings.HasPrefix(f, prefix) {
			return &fs.PathError{Op: "remove", Path: p, Err: errors.New("directory not empty")}
		}
	}
	for d := range m.dirs {
		if strings.HasPrefix(d, prefix) {
			return &fs.PathError{Op: "remove", Path: p, Err: errors.New("directory not empty")}
		}
	}
	delete(m.dirs, p)
	return nil
}

// The fs module's operations, on the file system held in memory when the
// run has one, else on the disk.

func (in *Interp) readPath(v Value, at ir.Pos) (data []byte, err error) {
	p := in.path(v, at)
	if m := in.rt.memfs; m != nil {
		return m.readFile(p)
	}
	in.blocking(func(context.Context) { data, err = os.ReadFile(p) })
	return data, err
}

func (in *Interp) writePath(v Value, data []byte, appending bool, at ir.Pos) (err error) {
	p := in.path(v, at)
	if m := in.rt.memfs; m != nil {
		return m.writeFile(p, data, appending)
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appending {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	in.blocking(func(context.Context) {
		var f *os.File
		f, err = os.OpenFile(p, flags, 0o644)
		if err != nil {
			return
		}
		_, err = f.Write(data)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	})
	return err
}

func (in *Interp) pathExists(v Value, at ir.Pos) bool {
	p := in.path(v, at)
	if m := in.rt.memfs; m != nil {
		return m.exists(p)
	}
	_, err := os.Stat(p)
	return err == nil
}

func (in *Interp) listPath(v Value, at ir.Pos) ([]string, error) {
	p := in.path(v, at)
	if m := in.rt.memfs; m != nil {
		return m.list(p)
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Strings(names)
	return names, nil
}

func (in *Interp) makeDirPath(v Value, at ir.Pos) error {
	p := in.path(v, at)
	if m := in.rt.memfs; m != nil {
		return m.makeDir(p)
	}
	return os.MkdirAll(p, 0o755)
}

func (in *Interp) removePath(v Value, at ir.Pos) error {
	p := in.path(v, at)
	if m := in.rt.memfs; m != nil {
		return m.remove(p)
	}
	return os.Remove(p)
}
