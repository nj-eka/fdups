// +build aix darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package filestat

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// fileStat implements FileStat interface for *nix os
type fileStat struct {
	path       string
	fileInfo   os.FileInfo
	sys        *syscall.Stat_t // *nix specific
	user       *user.User
	group      *user.Group
	symlink    FileStat
	metaKey    string
	sortingKey string
	repr       string
	prior      string
}

func (fs *fileStat) Path() string { return fs.path }

func (fs *fileStat) Inode() Inode { return Inode(fs.sys.Ino) }

func (fs *fileStat) Size() int64 { return fs.fileInfo.Size() }

func (fs *fileStat) Blksize() int64 { return fs.sys.Blksize }

func (fs *fileStat) Blocks() int64 {
	//return filestat.sys.Blocks
	return int64(math.Ceil(float64(fs.Size() / fs.Blksize())))
}

func (fs *fileStat) Perm() fs.FileMode { return fs.fileInfo.Mode().Perm() }

func (fs *fileStat) IsRegular() bool { return fs.fileInfo.Mode().IsRegular() }

func (fs *fileStat) BaseName() string { return fs.fileInfo.Name() }

func (fs *fileStat) ModTime() time.Time { return fs.fileInfo.ModTime() }

func (fs *fileStat) User() *user.User { return fs.user }

func (fs *fileStat) Group() *user.Group { return fs.group }

func (fs *fileStat) Symlink() FileStat { return fs.symlink }

func (fs *fileStat) MetaKey() string { return fs.metaKey }

func (fs *fileStat) String() string {
	if fs.repr == "" {
		path := fs.path
		if fs.symlink != nil {
			path = fmt.Sprintf("%s -> %s", fs.symlink.Path(), path)
		}
		ln := fs.Symlink()
		ino := fs.Inode()
		nlink := fs.sys.Nlink
		if ln != nil {
			ino = ln.Inode()
			nlink = ln.(*fileStat).sys.Nlink
		}
		fs.repr = fmt.Sprintf(
			"%10d(%2d)|%10s|%12d|%26s|%s:%s|%s",
			ino,
			nlink,
			fs.Perm(),
			fs.Size(),
			fs.ModTime().Format(time.RFC1123),
			fs.User().Username,
			fs.Group().Name,
			path,
		)
	}
	return fs.repr
}

func (fs *fileStat) Prior() string {
	return fs.prior
}

func (fs *fileStat) SortingKey() string {
	if fs.sortingKey == "" {
		t := "0"
		if fs.symlink != nil {
			t = "1"
		}
		fs.sortingKey = fmt.Sprintf("%2s%1s%19%3d%8s%8s%s",
			fs.prior,
			t,
			fs.ModTime().Format("20060102_1504050000"),
			strings.Count(os.Args[0], string(filepath.Separator)),
			fs.user.Uid,
			fs.group.Gid,
			fs.path, // todo: if fs.symlink != nil use fs.symlink.path ...
		)
	}
	return fs.sortingKey
}

//newFileStat initializes FileStat for *nix os
func newFileStat(path string, fileInfo os.FileInfo, metaKeyFunc MetaKeyFunc, priorFunc PriorFunc, symlink FileStat) (FileStat, error) {
	sys := fileInfo.Sys().(*syscall.Stat_t)
	userOwner, err := user.LookupId(fmt.Sprint(sys.Uid))
	if err != nil {
		return nil, err
	}
	groupOwner, err := user.LookupGroupId(fmt.Sprint(sys.Gid))
	if err != nil {
		return nil, err
	}
	fS := fileStat{
		path:     path,
		fileInfo: fileInfo,
		sys:      sys,
		user:     userOwner,
		group:    groupOwner,
		symlink:  symlink,
	}
	if metaKeyFunc != nil { // fnMetaKey is nil for symlink
		//if symlink != nil {
		fS.metaKey = metaKeyFunc(&fS)
	}
	if priorFunc != nil {
		fS.prior = priorFunc(path)
	}
	return &fS, nil
}

// todo: add windows support
// e.g.:
//if runtime.GOOS == "windows" {
//	fileinfo, _ := os.Stat(path)
//	stat := fileinfo.Sys().(*syscall.Win32FileAttributeData)
//	...
//} else {
//	fileinfo, _ := os.Stat(path)
//	stat = fileinfo.Sys().(*syscall.Stat_t)
//	...
//}
