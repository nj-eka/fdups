package filestat

import (
	"math"
)

type FileStatValidatorFunc func(fs FileStat) bool

// todo: make validators stack
func NewRegularSizeStatValidator(minSize, maxSize int64) FileStatValidatorFunc {
	if minSize < 0 {
		maxSize = 0
	}
	if maxSize < 0 {
		maxSize = math.MaxInt64 // 1<<63 - 1 || int64(^uint64(0) >> 1)
	}
	if maxSize < minSize {
		minSize, maxSize = maxSize, minSize
	}
	return func(fs FileStat) bool {
		size := fs.Size()
		return fs.IsRegular() && (size >= minSize) && (size <= maxSize)
	}
}

//func NewRegularSizeModifiedAfterStatValidator(minSize, maxSize int64, modifiedAfter time.Time) FileStatValidatorFunc {
//	if minSize < 0 {
//		maxSize = 0
//	}
//	if maxSize < 0 {
//		maxSize = math.MaxInt64
//	}
//	if maxSize < minSize {
//		minSize, maxSize = maxSize, minSize
//	}
//	return func(fs FileStat) bool {
//		size := fs.Size()
//		return fs.IsRegular() && (size >= minSize) && (size <= maxSize) && (fs.ModTime().After(modifiedAfter))
//	}
//}
