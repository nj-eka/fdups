package filestat

import "fmt"

// FileStatMetaKeyFunc - func type for meta key builder (used in FileStat.MetaKey())
// see NewMetaKeyFunc
type MetaKeyFunc func(fs FileStat) string

// NewMetaKeyFunc customizes meta key builder function (enclosure for FileStatMetaKeyFunc)
// todo: make keys set order to be also customizable - thus, it will be possible to set sorting order by meta key.
func NewMetaKeyFunc(bSize bool, bName bool, bPerm bool, bUID bool, bGID bool, bModTime bool) MetaKeyFunc {
	fnFilterAny := func(FileStat) interface{} { return "*" }
	fnFilters := [...]func(FileStat) interface{}{fnFilterAny, fnFilterAny, fnFilterAny, fnFilterAny, fnFilterAny, fnFilterAny}
	i := 0
	if bSize {
		fnFilters[i] = func(fs FileStat) interface{} { return fs.Size() }
	}
	i++
	if bModTime {
		fnFilters[i] = func(fs FileStat) interface{} {
			return fs.ModTime().UnixNano()
			// return filestat.ModTime().Format(time.RFC3339Nano)
		}
	}
	i++
	if bUID {
		fnFilters[i] = func(fs FileStat) interface{} { return fs.User().Uid }
	}
	i++
	if bGID {
		fnFilters[i] = func(fs FileStat) interface{} { return fs.Group().Gid }
	}
	i++
	if bPerm {
		fnFilters[i] = func(fs FileStat) interface{} { return fmt.Sprintf("%#o", fs.Perm()) }
	}
	i++
	if bName {
		fnFilters[i] = func(fs FileStat) interface{} {
			if s := fs.Symlink(); s != nil {
				fs = s
			}
			return fs.BaseName()
		}
	}
	return func(fs FileStat) string {
		results := make([]interface{}, len(fnFilters))
		for i, fn := range fnFilters {
			results[i] = fn(fs)
		}
		return fmt.Sprintf("size:%12v;mt:%v;uid:%s;gid:%s;perm:%v;name:%s", results[:]...)
	}
}
