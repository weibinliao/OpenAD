package mocks

import (
	"io/fs"
	"os"
	"time"
)

type FileSystem struct {
	StatEntries    map[string]os.FileInfo
	StatErrors     map[string]error
	ReadDirEntries map[string][]os.DirEntry
	ReadDirErrors  map[string]error
}

type fileInfo struct {
	name string
	mode os.FileMode
}

type dirEntry struct {
	name string
	mode os.FileMode
}

func NewFileSystem() *FileSystem {
	return &FileSystem{
		StatEntries:    make(map[string]os.FileInfo),
		StatErrors:     make(map[string]error),
		ReadDirEntries: make(map[string][]os.DirEntry),
		ReadDirErrors:  make(map[string]error),
	}
}

func (fileSystem *FileSystem) Stat(path string) (os.FileInfo, error) {
	if err, ok := fileSystem.StatErrors[path]; ok {
		return nil, err
	}

	if entry, ok := fileSystem.StatEntries[path]; ok {
		return entry, nil
	}

	return nil, fs.ErrNotExist
}

func (fileSystem *FileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	if err, ok := fileSystem.ReadDirErrors[path]; ok {
		return nil, err
	}

	entries := fileSystem.ReadDirEntries[path]
	return append([]os.DirEntry(nil), entries...), nil
}

func DirectoryInfo(name string) os.FileInfo {
	return fileInfo{name: name, mode: os.ModeDir}
}

func FileInfo(name string) os.FileInfo {
	return fileInfo{name: name}
}

func DirectoryEntry(name string) os.DirEntry {
	return dirEntry{name: name, mode: os.ModeDir}
}

func FileEntry(name string) os.DirEntry {
	return dirEntry{name: name}
}

func SymlinkEntry(name string) os.DirEntry {
	return dirEntry{name: name, mode: os.ModeSymlink}
}

func (info fileInfo) Name() string       { return info.name }
func (info fileInfo) Size() int64        { return 0 }
func (info fileInfo) Mode() os.FileMode  { return info.mode }
func (info fileInfo) ModTime() time.Time { return time.Time{} }
func (info fileInfo) IsDir() bool        { return info.mode.IsDir() }
func (info fileInfo) Sys() any           { return nil }

func (entry dirEntry) Name() string      { return entry.name }
func (entry dirEntry) IsDir() bool       { return entry.mode.IsDir() }
func (entry dirEntry) Type() os.FileMode { return entry.mode }
func (entry dirEntry) Info() (os.FileInfo, error) {
	return fileInfo{name: entry.name, mode: entry.mode}, nil
}
