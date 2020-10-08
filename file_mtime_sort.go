package main

// Provide sort-by-mtime of a File map, using sort.Interface

// FileMtimeSort is a slice of File
type FileMtimeSort []*File

// NewFileMtimeSort crate a new FileMtimeSort from a File map
func NewFileMtimeSort(files FileMap) FileMtimeSort {
	slice := make(FileMtimeSort, 0, len(files))

	for _, file := range files {
		slice = append(slice, file)
	}
	return slice
}

// Len return the len of the slice (needed by sort.Interface)
func (d FileMtimeSort) Len() int {
	return len(d)
}

// Swap entries (needed by sort.Interface)
func (d FileMtimeSort) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

// Less returns true if ModTime of i is before j (needed by sort.Interface)
func (d FileMtimeSort) Less(i, j int) bool {
	return d[i].ModTime.Before(d[j].ModTime)
}
