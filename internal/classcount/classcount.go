package classcount

import (
	"archive/zip"
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const jimageMagic = 0xCAFEDADA

func JVM(home string) int {
	for _, p := range []string{"lib/modules", "jre/lib/modules"} {
		if n := jimage(filepath.Join(home, p)); n > 0 {
			return n
		}
	}
	return Tree(filepath.Join(home, "lib")) + Tree(filepath.Join(home, "jre", "lib"))
}

func jimage(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	header := make([]byte, 16)
	if _, err := f.ReadAt(header, 0); err != nil {
		return 0
	}
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		if order.Uint32(header[0:4]) == jimageMagic {
			return int(order.Uint32(header[12:16]))
		}
	}
	return 0
}

func Tree(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".class"):
			count++
		case strings.HasSuffix(path, ".jar"), strings.HasSuffix(path, ".war"):
			count += Archive(path)
		}
		return nil
	})
	return count
}

func Archive(path string) int {
	z, err := zip.OpenReader(path)
	if err != nil {
		return 0
	}
	defer z.Close()
	count := 0
	for _, f := range z.File {
		switch {
		case strings.HasSuffix(f.Name, ".class"):
			count++
		case strings.HasSuffix(f.Name, ".jar"):
			count += nested(f)
		}
	}
	return count
}

func nested(f *zip.File) int {
	if f.Method != zip.Store {
		return 0
	}
	r, err := f.OpenRaw()
	if err != nil {
		return 0
	}
	ra, ok := r.(interface {
		ReadAt([]byte, int64) (int, error)
	})
	if !ok {
		return 0
	}
	z, err := zip.NewReader(ra, int64(f.CompressedSize64))
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range z.File {
		if strings.HasSuffix(e.Name, ".class") {
			count++
		}
	}
	return count
}
