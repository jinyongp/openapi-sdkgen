package sdkgen

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

// referenceSourceFS exposes only containment-validated reference files and reads
// their bytes from decodedSourceCache snapshots. libopenapi therefore indexes
// the same immutable content that compiler preflight inspected.
type referenceSourceFS struct {
	files map[string]referenceFSFile
	dirs  map[string][]fs.DirEntry
}

type referenceFSFile struct {
	data []byte
	info referenceFSInfo
}

func newReferenceSourceFS(root string, filters []string, cache *decodedSourceCache, session *compatibilitySession) (*referenceSourceFS, error) {
	result := &referenceSourceFS{
		files: make(map[string]referenceFSFile),
		dirs:  map[string][]fs.DirEntry{".": {}},
	}
	children := map[string]map[string]fs.DirEntry{".": {}}
	for _, filter := range filters {
		name := path.Clean(filepath.ToSlash(filter))
		if name == "." || name == ".openapi-sdkgen-no-local-references" {
			continue
		}
		if !fs.ValidPath(name) {
			return nil, fmt.Errorf("invalid OpenAPI reference file filter %q", filter)
		}
		absolute := filepath.Join(root, filepath.FromSlash(name))
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return nil, fmt.Errorf("resolve OpenAPI reference source %s: %w", absolute, err)
		}
		source, err := cache.load(resolved)
		if err != nil {
			return nil, fmt.Errorf("load OpenAPI reference source %s: %w", resolved, err)
		}
		effective, err := session.effectiveSource(resolved, source, session.sourceContext(resolved))
		if err != nil {
			return nil, err
		}
		stat, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("stat OpenAPI reference source %s: %w", resolved, err)
		}
		info := referenceFSInfo{
			name:    path.Base(name),
			size:    int64(len(effective.data)),
			mode:    stat.Mode(),
			modTime: stat.ModTime(),
		}
		result.files[name] = referenceFSFile{data: effective.data, info: info}
		ensureReferenceFSParents(children, name)
		parent := path.Dir(name)
		if parent == "" {
			parent = "."
		}
		children[parent][path.Base(name)] = fs.FileInfoToDirEntry(info)
	}
	for directory, entries := range children {
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		sort.Strings(names)
		result.dirs[directory] = make([]fs.DirEntry, 0, len(names))
		for _, name := range names {
			result.dirs[directory] = append(result.dirs[directory], entries[name])
		}
	}
	return result, nil
}

func ensureReferenceFSParents(children map[string]map[string]fs.DirEntry, name string) {
	current := path.Dir(name)
	for current != "." && current != "" {
		if children[current] == nil {
			children[current] = map[string]fs.DirEntry{}
		}
		parent := path.Dir(current)
		if parent == "" {
			parent = "."
		}
		if children[parent] == nil {
			children[parent] = map[string]fs.DirEntry{}
		}
		base := path.Base(current)
		children[parent][base] = fs.FileInfoToDirEntry(referenceFSInfo{name: base, mode: fs.ModeDir | 0o555})
		current = parent
	}
}

func (filesystem *referenceSourceFS) Open(name string) (fs.File, error) {
	name = path.Clean(name)
	if name == "." {
		return &referenceFSDir{name: ".", entries: filesystem.dirs["."]}, nil
	}
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if file, ok := filesystem.files[name]; ok {
		return &referenceFSDataFile{Reader: *bytes.NewReader(file.data), info: file.info}, nil
	}
	if entries, ok := filesystem.dirs[name]; ok {
		return &referenceFSDir{name: path.Base(name), entries: entries}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (filesystem *referenceSourceFS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = path.Clean(name)
	entries, ok := filesystem.dirs[name]
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	return append([]fs.DirEntry(nil), entries...), nil
}

func (filesystem *referenceSourceFS) Stat(name string) (fs.FileInfo, error) {
	name = path.Clean(name)
	if name == "." {
		return referenceFSInfo{name: ".", mode: fs.ModeDir | 0o555}, nil
	}
	if file, ok := filesystem.files[name]; ok {
		return file.info, nil
	}
	if _, ok := filesystem.dirs[name]; ok {
		return referenceFSInfo{name: path.Base(name), mode: fs.ModeDir | 0o555}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

type referenceFSInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (info referenceFSInfo) Name() string       { return info.name }
func (info referenceFSInfo) Size() int64        { return info.size }
func (info referenceFSInfo) Mode() fs.FileMode  { return info.mode }
func (info referenceFSInfo) ModTime() time.Time { return info.modTime }
func (info referenceFSInfo) IsDir() bool        { return info.mode.IsDir() }
func (info referenceFSInfo) Sys() any           { return nil }

type referenceFSDataFile struct {
	bytes.Reader
	info referenceFSInfo
}

func (file *referenceFSDataFile) Close() error               { return nil }
func (file *referenceFSDataFile) Stat() (fs.FileInfo, error) { return file.info, nil }

type referenceFSDir struct {
	name    string
	entries []fs.DirEntry
	offset  int
}

func (directory *referenceFSDir) Close() error { return nil }
func (directory *referenceFSDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: directory.name, Err: fs.ErrInvalid}
}
func (directory *referenceFSDir) Stat() (fs.FileInfo, error) {
	return referenceFSInfo{name: directory.name, mode: fs.ModeDir | 0o555}, nil
}
func (directory *referenceFSDir) ReadDir(count int) ([]fs.DirEntry, error) {
	if directory.offset >= len(directory.entries) {
		if count > 0 {
			return nil, io.EOF
		}
		return []fs.DirEntry{}, nil
	}
	end := len(directory.entries)
	if count > 0 && directory.offset+count < end {
		end = directory.offset + count
	}
	result := append([]fs.DirEntry(nil), directory.entries[directory.offset:end]...)
	directory.offset = end
	return result, nil
}
