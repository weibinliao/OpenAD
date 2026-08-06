package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/testutil/mocks"
)

func TestScanDirectoryTraversesTreeAndFiltersInheritedPermissions(t *testing.T) {
	fileSystem := mocks.NewFileSystem()
	fileSystem.StatEntries[`C:\Finance`] = mocks.DirectoryInfo("Finance")
	fileSystem.ReadDirEntries[`C:\Finance`] = []os.DirEntry{
		mocks.DirectoryEntry("Reports"),
		mocks.FileEntry("summary.txt"),
	}
	fileSystem.ReadDirEntries[`C:\Finance\Reports`] = []os.DirEntry{}

	permissionReader := &stubPermissionReader{
		PermissionsByPath: map[string][]Permission{
			`C:\Finance`: {
				{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, TrusteeSID: "S-1-5-21-100", Rights: "Read", Inherited: false},
				{Path: `C:\Finance`, Trustee: `DOMAIN\Everyone`, TrusteeSID: "S-1-1-0", Rights: "Read", Inherited: true},
			},
			`C:\Finance\Reports`: {
				{Path: `C:\Finance\Reports`, Trustee: `DOMAIN\Bob`, TrusteeSID: "S-1-5-21-101", Rights: "Write", Inherited: false},
			},
			`C:\Finance\summary.txt`: {
				{Path: `C:\Finance\summary.txt`, Trustee: `DOMAIN\Carol`, TrusteeSID: "S-1-5-21-102", Rights: "Read", Inherited: false},
			},
		},
	}

	progressEvents := make([]Progress, 0, 3)
	scanner := NewNTFSScannerWithDependencies(fileSystem, permissionReader)
	result, err := scanner.ScanDirectory(`C:\Finance`, Options{
		MaxDepth:         2,
		IncludeInherited: false,
		Progress: func(progress Progress) {
			progressEvents = append(progressEvents, progress)
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, `C:\Finance`, result.RootPath)
	assert.Equal(t, 2, result.ItemsScanned)
	assert.Len(t, result.Permissions, 2)
	assert.Empty(t, result.Skipped)
	require.Len(t, progressEvents, 2)
	assert.Equal(t, 2, progressEvents[len(progressEvents)-1].ItemsScanned)
	assert.Equal(t, 2, progressEvents[len(progressEvents)-1].PermissionCount)
}

func TestScanDirectoryCollectsSkippedEntriesAndPermissionErrors(t *testing.T) {
	fileSystem := mocks.NewFileSystem()
	fileSystem.StatEntries[`C:\Finance`] = mocks.DirectoryInfo("Finance")
	fileSystem.ReadDirEntries[`C:\Finance`] = []os.DirEntry{
		mocks.SymlinkEntry("Loop"),
		mocks.FileEntry("blocked.txt"),
	}

	permissionReader := &stubPermissionReader{
		ErrorsByPath: map[string]error{
			`C:\Finance\blocked.txt`: errors.New("access denied"),
		},
	}

	scanner := NewNTFSScannerWithDependencies(fileSystem, permissionReader)
	result, err := scanner.ScanDirectory(`C:\Finance`, Options{MaxDepth: 1, IncludeInherited: true})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.ItemsScanned)
	assert.Len(t, result.Skipped, 1)
	assert.Equal(t, `C:\Finance\Loop`, result.Skipped[0].Path)
}

func TestScanDirectoryReturnsValidationAndStatErrors(t *testing.T) {
	fileSystem := mocks.NewFileSystem()
	fileSystem.StatErrors[`C:\Missing`] = errors.New("path not found")
	scanner := NewNTFSScannerWithDependencies(fileSystem, &stubPermissionReader{})

	result, err := scanner.ScanDirectory("", Options{})
	assert.Nil(t, result)
	assert.EqualError(t, err, "scan path is required")

	result, err = scanner.ScanDirectory(`C:\Missing`, Options{})
	assert.Nil(t, result)
	assert.EqualError(t, err, `stat C:\Missing: path not found`)
}

func TestScanDirectoryAnnotatesPermissionDeniedUNCPaths(t *testing.T) {
	fileSystem := mocks.NewFileSystem()
	fileSystem.StatErrors[`\\files.example.com\software`] = os.ErrPermission
	scanner := NewNTFSScannerWithDependencies(fileSystem, &stubPermissionReader{})

	result, err := scanner.ScanDirectory(`\\files.example.com\software`, Options{})
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
	assert.Contains(t, err.Error(), "UNC scanning uses the selected UNC access identity")
}

func TestScanDirectoryRespectsCancellation(t *testing.T) {
	fileSystem := mocks.NewFileSystem()
	fileSystem.StatEntries[`C:\Finance`] = mocks.DirectoryInfo("Finance")
	fileSystem.ReadDirEntries[`C:\Finance`] = []os.DirEntry{mocks.DirectoryEntry("Reports")}
	permissionReader := &stubPermissionReader{
		PermissionsByPath: map[string][]Permission{
			`C:\Finance`: {{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, Rights: "Read"}},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scanner := NewNTFSScannerWithDependencies(fileSystem, permissionReader)
	result, err := scanner.ScanDirectory(`C:\Finance`, Options{Context: ctx})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestNewNTFSScannerUsesDefaultDependencies(t *testing.T) {
	scanner := NewNTFSScanner()
	require.NotNil(t, scanner)
	require.NotNil(t, scanner.fileSystem)
	require.NotNil(t, scanner.permissionReader)
}

func TestOptionsHelpersAndDefaultReaders(t *testing.T) {
	normalized := (Options{MaxDepth: -10}).normalize()
	assert.Equal(t, -1, normalized.MaxDepth)
	assert.True(t, normalized.shouldDescend(100))
	assert.False(t, (Options{MaxDepth: 1}).shouldDescend(1))
	assert.NotPanics(t, func() {
		Options{}.emitProgress(Progress{ItemsScanned: 1})
	})
}

func TestDefaultFileSystemAndPermissionReader(t *testing.T) {
	rootPath := t.TempDir()
	childPath := filepath.Join(rootPath, "child.txt")
	require.NoError(t, os.WriteFile(childPath, []byte("hello"), 0o644))

	fileSystem := osFileSystem{}
	info, err := fileSystem.Stat(rootPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	entries, err := fileSystem.ReadDir(rootPath)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "child.txt", entries[0].Name())

	permissions, err := ntfsPermissionReader{scanner: NewNTFSScanner()}.ReadPermissions(childPath)
	if runtime.GOOS == "windows" {
		require.NoError(t, err)
		assert.NotNil(t, permissions)
	} else {
		assert.Nil(t, permissions)
		assert.Error(t, err)
	}
}

type stubPermissionReader struct {
	PermissionsByPath map[string][]Permission
	ErrorsByPath      map[string]error
}

func (reader *stubPermissionReader) ReadPermissions(path string) ([]Permission, error) {
	if err, ok := reader.ErrorsByPath[path]; ok {
		return nil, err
	}

	permissions := reader.PermissionsByPath[path]
	return append([]Permission(nil), permissions...), nil
}
