// Package filestat defines and implements basic file statistics interface
package filestat

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

type Inode uint64

// FileStat describes a file (use GetFileStat)
type FileStat interface {
	// Path - full path (abs cleaned resolved)
	Path() string
	// BaseName - file name wo dirs
	BaseName() string
	// Inode - Unix inode (or analogue) used here to resolve multiple links to the same file content
	Inode() Inode
	// IsRegular - checks whether file is regular (FileMode & ModeType == 0)
	IsRegular() bool
	// Size - content size
	Size() int64
	// Blksize - block size of file (file system specific)
	Blksize() int64
	// Blocks - number of blocks occupied by file (here it's calculated as Blocks = Ceil (Size / Blksize) )
	Blocks() int64
	// ModTime - modification time
	ModTime() time.Time
	// Perm - Unix permission bits (or analogue)
	Perm() fs.FileMode
	// User - user owner of file
	User() *user.User
	// Group - group owner of file
	Group() *user.Group
	// Symlink - initial (from search matching) link to this file
	Symlink() FileStat
	// MetaKey - custom key based on file metadata (such as name, size, uid:gid, perm, ...)
	// Note: content itself is ignored - see FileStatMetaKeyFunc for details
	MetaKey() string
	// String - pretty string for view (like ls -i)
	String() string
	//
	Prior() string
	// SortingKey - sorting key for output
	SortingKey() string
}

// GetFileStat - FileStat builder function (uses os specific func newFileStat)
func GetFileStat(path string, metaKeyFunc MetaKeyFunc, priorFunc PriorFunc, SymLinkEnabled bool) (FileStat, error) {
	if fileInfo, err := os.Lstat(path); err == nil {
		if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			if SymLinkEnabled {
				targetPath, err := filepath.EvalSymlinks(path)
				if err != nil {
					return nil, fmt.Errorf("unresolved symlink [%s]: %w", path, err)
				}
				targetInfo, err := os.Stat(targetPath)
				if err != nil {
					return nil, fmt.Errorf("getting target file [%s] Stat for symlink [%s] failed: %w", targetPath, path, err)
				}
				sfs, err := newFileStat(path, fileInfo, nil, priorFunc, nil)
				if err != nil {
					return nil, fmt.Errorf("getting FileStat for symlink [%s] failed: %w", path, err)
				}
				return newFileStat(targetPath, targetInfo, metaKeyFunc, priorFunc, sfs)
			} else {
				return nil, fmt.Errorf("symlink processing is disabled [%s]", path) // info
			}
		} else {
			return newFileStat(path, fileInfo, metaKeyFunc, priorFunc, nil)
		}
	} else {
		// errors.Is(err, os.ErrNotExist)
		return nil, fmt.Errorf("getting FileStat for file [%s] failed: %w", path, err)
	}
}
