package net

import (
	"fmt"
	"hash/fnv"
	"io/fs"
	"sort"
)

// DataID hashes the embedded content plus the protocol version; peers must match to play together.
func DataID(dataFS fs.FS) string {
	h := fnv.New64a()
	var names []string
	fs.WalkDir(dataFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return nil
	})
	sort.Strings(names)
	for _, n := range names {
		b, _ := fs.ReadFile(dataFS, n)
		h.Write([]byte(n))
		h.Write(b)
	}
	return fmt.Sprintf("%x-%d", h.Sum64(), Protocol)
}
