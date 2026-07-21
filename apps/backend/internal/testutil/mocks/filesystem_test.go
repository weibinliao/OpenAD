package mocks

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSystemReturnsConfiguredEntriesAndErrors(t *testing.T) {
	fileSystem := NewFileSystem()
	fileSystem.StatEntries[`C:\Finance`] = DirectoryInfo("Finance")
	fileSystem.ReadDirEntries[`C:\Finance`] = []fs.DirEntry{DirectoryEntry("Reports"), FileEntry("summary.txt")}

	info, err := fileSystem.Stat(`C:\Finance`)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	entries, err := fileSystem.ReadDir(`C:\Finance`)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "Reports", entries[0].Name())
	assert.Equal(t, "summary.txt", entries[1].Name())

	_, err = fileSystem.Stat(`C:\Missing`)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestDirectoryEntriesExposeExpectedModes(t *testing.T) {
	assert.True(t, DirectoryEntry("Reports").IsDir())
	assert.False(t, FileEntry("summary.txt").IsDir())
	assert.NotZero(t, SymlinkEntry("Shortcut").Type()&fs.ModeSymlink)
	assert.True(t, DirectoryInfo("Reports").IsDir())
	assert.False(t, FileInfo("summary.txt").IsDir())
}
