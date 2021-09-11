package filestat

type FileSizeLesserFunc func(fs FileStat) bool

func NewFileSizeLesserFunc(maxSize int64, inBlocks bool) FileSizeLesserFunc {
	return func(fs FileStat) (result bool) {
		if inBlocks {
			return fs.Blocks() < maxSize
		}
		return fs.Size() < maxSize
	}
}
