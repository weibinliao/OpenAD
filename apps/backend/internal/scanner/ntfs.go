package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fileSystem interface {
	Stat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.DirEntry, error)
}

type permissionReader interface {
	ReadPermissions(path string) ([]Permission, error)
}

type NTFSScanner struct {
	fileSystem       fileSystem
	permissionReader permissionReader
}

type Options struct {
	MaxDepth         int             `json:"max_depth"`
	IncludeInherited bool            `json:"include_inherited"`
	Context          context.Context `json:"-"`
	Progress         ProgressFunc    `json:"-"`
}

type Progress struct {
	ItemsScanned    int    `json:"items_scanned"`
	PermissionCount int    `json:"permission_count"`
	CurrentPath     string `json:"current_path"`
}

type ProgressFunc func(Progress)

type Permission struct {
	Path                      string `json:"path"`
	Trustee                   string `json:"trustee"`
	TrusteeSID                string `json:"trustee_sid"`
	Rights                    string `json:"rights"`
	Type                      string `json:"type"`
	Inherited                 bool   `json:"inherited"`
	Source                    string `json:"source,omitempty"`
	AppliesTo                 string `json:"applies_to,omitempty"`
	AccountType               string `json:"account_type,omitempty"`
	AccessMask                string `json:"access_mask,omitempty"`
	RiskLevel                 string `json:"risk_level,omitempty"`
	ParentDelta               string `json:"parent_delta,omitempty"`
	AccountName               string `json:"account_name,omitempty"`
	FirstName                 string `json:"first_name,omitempty"`
	LastName                  string `json:"last_name,omitempty"`
	Email                     string `json:"email,omitempty"`
	Department                string `json:"department,omitempty"`
	Division                  string `json:"division,omitempty"`
	Domain                    string `json:"domain,omitempty"`
	OriginatingGroup          string `json:"originating_group,omitempty"`
	GroupInheritanceHierarchy string `json:"group_inheritance_hierarchy,omitempty"`
	ResolutionSource          string `json:"resolution_source,omitempty"`
	ResolutionReason          string `json:"resolution_reason,omitempty"`
}

type PathError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type Result struct {
	RootPath         string       `json:"root_path"`
	MaxDepth         int          `json:"max_depth"`
	IncludeInherited bool         `json:"include_inherited"`
	ItemsScanned     int          `json:"items_scanned"`
	Permissions      []Permission `json:"permissions"`
	Skipped          []PathError  `json:"skipped,omitempty"`
}

type scanTarget struct {
	path  string
	depth int
	isDir bool
}

func NewNTFSScanner() *NTFSScanner {
	return NewNTFSScannerWithDependencies(nil, nil)
}

func NewNTFSScannerWithDependencies(fileSystem fileSystem, permissionReader permissionReader) *NTFSScanner {
	scanner := &NTFSScanner{}
	if fileSystem == nil {
		fileSystem = osFileSystem{}
	}

	if permissionReader == nil {
		permissionReader = ntfsPermissionReader{scanner: scanner}
	}

	scanner.fileSystem = fileSystem
	scanner.permissionReader = permissionReader

	return scanner
}

func (scanner *NTFSScanner) ScanDirectory(path string, options Options) (*Result, error) {
	if path == "" {
		return nil, fmt.Errorf("scan path is required")
	}

	if err := options.contextErr(); err != nil {
		return nil, err
	}

	cleanPath := filepath.Clean(path)
	info, err := scanner.fileSystem.Stat(cleanPath)
	if err != nil {
		return nil, wrapScanPathError("stat", cleanPath, err)
	}

	normalized := options.normalize()
	result := &Result{
		RootPath:         cleanPath,
		MaxDepth:         normalized.MaxDepth,
		IncludeInherited: normalized.IncludeInherited,
		Permissions:      make([]Permission, 0, 128),
	}

	stack := []scanTarget{{
		path:  cleanPath,
		depth: 0,
		isDir: info.IsDir(),
	}}

	for len(stack) > 0 {
		if err := options.contextErr(); err != nil {
			return nil, err
		}

		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		result.ItemsScanned++

		if err := options.contextErr(); err != nil {
			return nil, err
		}

		permissions, err := scanner.permissionReader.ReadPermissions(current.path)
		if err != nil {
			result.Skipped = append(result.Skipped, PathError{
				Path:  current.path,
				Error: err.Error(),
			})
			normalized.emitProgress(Progress{
				ItemsScanned:    result.ItemsScanned,
				PermissionCount: len(result.Permissions),
				CurrentPath:     current.path,
			})
			continue
		}

		result.Permissions = append(result.Permissions, filterPermissions(permissions, normalized.IncludeInherited)...)

		if !current.isDir || !normalized.shouldDescend(current.depth) {
			normalized.emitProgress(Progress{
				ItemsScanned:    result.ItemsScanned,
				PermissionCount: len(result.Permissions),
				CurrentPath:     current.path,
			})
			continue
		}

		if err := options.contextErr(); err != nil {
			return nil, err
		}

		children, skipped, err := scanner.listChildren(current, options.Context)
		if err != nil {
			result.Skipped = append(result.Skipped, PathError{
				Path:  current.path,
				Error: err.Error(),
			})
			normalized.emitProgress(Progress{
				ItemsScanned:    result.ItemsScanned,
				PermissionCount: len(result.Permissions),
				CurrentPath:     current.path,
			})
			continue
		}

		result.Skipped = append(result.Skipped, skipped...)
		stack = append(stack, children...)
		normalized.emitProgress(Progress{
			ItemsScanned:    result.ItemsScanned,
			PermissionCount: len(result.Permissions),
			CurrentPath:     current.path,
		})
	}

	return result, nil
}

func (scanner *NTFSScanner) listChildren(target scanTarget, ctx context.Context) ([]scanTarget, []PathError, error) {
	if err := contextErrFromOptions(ctx); err != nil {
		return nil, nil, err
	}

	entries, err := scanner.fileSystem.ReadDir(target.path)
	if err != nil {
		return nil, nil, wrapScanPathError("read directory", target.path, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	children := make([]scanTarget, 0, len(entries))
	skipped := make([]PathError, 0)

	for _, entry := range entries {
		if err := contextErrFromOptions(ctx); err != nil {
			return nil, nil, err
		}

		childPath := filepath.Join(target.path, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			skipped = append(skipped, PathError{
				Path:  childPath,
				Error: "skipped symbolic link to avoid traversal loops",
			})
			continue
		}

		if !entry.IsDir() {
			continue
		}

		children = append(children, scanTarget{
			path:  childPath,
			depth: target.depth + 1,
			isDir: entry.IsDir(),
		})
	}

	return children, skipped, nil
}

func (options Options) normalize() Options {
	if options.MaxDepth < -1 {
		options.MaxDepth = -1
	}

	return options
}

func (options Options) shouldDescend(depth int) bool {
	return options.MaxDepth < 0 || depth < options.MaxDepth
}

func (options Options) emitProgress(progress Progress) {
	if options.Progress == nil {
		return
	}

	options.Progress(progress)
}

func (options Options) contextErr() error {
	return contextErrFromOptions(options.Context)
}

func contextErrFromOptions(ctx context.Context) error {
	if ctx == nil {
		return nil
	}

	return ctx.Err()
}

func wrapScanPathError(operation, path string, err error) error {
	if err == nil {
		return nil
	}

	if isPathAccessDenied(err) {
		if isUNCPath(path) {
			return fmt.Errorf("%s %s: access denied (%w; UNC scanning uses the selected UNC access identity)", operation, path, err)
		}
		return fmt.Errorf("%s %s: access denied (%w)", operation, path, err)
	}

	return fmt.Errorf("%s %s: %w", operation, path, err)
}

func isPathAccessDenied(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, os.ErrPermission) ||
		strings.Contains(strings.ToLower(err.Error()), "access is denied") ||
		strings.Contains(strings.ToLower(err.Error()), "permission denied") ||
		strings.Contains(strings.ToLower(err.Error()), "user name or password is incorrect") ||
		strings.Contains(strings.ToLower(err.Error()), "specified network password is not correct") ||
		strings.Contains(strings.ToLower(err.Error()), "logon failure") ||
		strings.Contains(strings.ToLower(err.Error()), "unknown user name or bad password") ||
		strings.Contains(strings.ToLower(err.Error()), "the network password is not correct")
}

func isUNCPath(path string) bool {
	return strings.HasPrefix(strings.TrimSpace(path), `\\`)
}

func filterPermissions(permissions []Permission, includeInherited bool) []Permission {
	if includeInherited {
		return permissions
	}

	filtered := make([]Permission, 0, len(permissions))
	for _, permission := range permissions {
		if permission.Inherited {
			continue
		}

		filtered = append(filtered, permission)
	}

	return filtered
}

type osFileSystem struct{}

func (osFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (osFileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

type ntfsPermissionReader struct {
	scanner *NTFSScanner
}

func (reader ntfsPermissionReader) ReadPermissions(path string) ([]Permission, error) {
	return reader.scanner.readPermissions(path)
}
