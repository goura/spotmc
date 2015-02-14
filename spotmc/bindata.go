package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"os"
	"time"
	"io/ioutil"
	"path"
	"path/filepath"
)

func bindata_read(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindata_file_info struct {
	name string
	size int64
	mode os.FileMode
	modTime time.Time
}

func (fi bindata_file_info) Name() string {
	return fi.name
}
func (fi bindata_file_info) Size() int64 {
	return fi.size
}
func (fi bindata_file_info) Mode() os.FileMode {
	return fi.mode
}
func (fi bindata_file_info) ModTime() time.Time {
	return fi.modTime
}
func (fi bindata_file_info) IsDir() bool {
	return false
}
func (fi bindata_file_info) Sys() interface{} {
	return nil
}

var _data_initscript_sh = []byte("\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\x8c\x8f\xc1\x6e\xc2\x30\x10\x44\xcf\xf1\x57\x4c\x43\x0f\xe5\x10\x19\xc4\x89\xf0\x35\xce\xb2\x10\x2b\xb6\xd7\xf2\x3a\xad\x50\xd5\x7f\x6f\x08\x52\x85\xc4\xa5\x7b\xda\x99\x9d\xa7\xd5\x6c\xde\xec\xe0\x93\x1d\x9c\x8e\x66\x03\x1a\x27\x92\x74\xf1\xd7\x1e\x07\x1c\x8f\xd8\xef\x16\x33\x17\x21\x56\x4d\x2e\x72\x8f\xf3\x1c\xe3\xad\xd3\x48\x9d\x56\xc9\x99\x8b\x31\x5a\x5d\xa9\x1f\x5b\x7c\x9b\xa6\xca\x4c\x23\xec\xa7\x2b\x36\x08\x4d\x56\xe7\x41\x6f\x6a\x5f\xa1\x9f\x3b\x26\xf9\x41\x95\x88\xee\xf2\x1f\xaa\x99\x7c\x08\x2e\x04\x68\x96\x1a\xc9\x34\x4c\xa3\xa0\x7d\x09\xf6\xf8\x72\xbe\xfa\x74\xc5\x61\x07\xe5\xa5\xd2\x59\x5b\xd3\x68\x60\xce\x8b\x75\xff\x4e\x4e\x19\xed\xfb\xbe\x85\x4f\x06\x58\x3b\x6c\x97\xe5\x31\xab\xfc\x53\xa7\xd3\x9a\x90\xfc\x1c\x90\xfc\x7c\x67\x75\x64\x7e\x03\x00\x00\xff\xff\xb0\xd8\x40\x33\x4c\x01\x00\x00")

func data_initscript_sh_bytes() ([]byte, error) {
	return bindata_read(
		_data_initscript_sh,
		"data/initscript.sh",
	)
}

func data_initscript_sh() (*asset, error) {
	bytes, err := data_initscript_sh_bytes()
	if err != nil {
		return nil, err
	}

	info := bindata_file_info{name: "data/initscript.sh", size: 332, mode: os.FileMode(420), modTime: time.Unix(1423908235, 0)}
	a := &asset{bytes: bytes, info:  info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"data/initscript.sh": data_initscript_sh,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for name := range node.Children {
		rv = append(rv, name)
	}
	return rv, nil
}

type _bintree_t struct {
	Func func() (*asset, error)
	Children map[string]*_bintree_t
}
var _bintree = &_bintree_t{nil, map[string]*_bintree_t{
	"data": &_bintree_t{nil, map[string]*_bintree_t{
		"initscript.sh": &_bintree_t{data_initscript_sh, map[string]*_bintree_t{
		}},
	}},
}}

// Restore an asset under the given directory
func RestoreAsset(dir, name string) error {
        data, err := Asset(name)
        if err != nil {
                return err
        }
        info, err := AssetInfo(name)
        if err != nil {
                return err
        }
        err = os.MkdirAll(_filePath(dir, path.Dir(name)), os.FileMode(0755))
        if err != nil {
                return err
        }
        err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
        if err != nil {
                return err
        }
        err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
        if err != nil {
                return err
        }
        return nil
}

// Restore assets under the given directory recursively
func RestoreAssets(dir, name string) error {
        children, err := AssetDir(name)
        if err != nil { // File
                return RestoreAsset(dir, name)
        } else { // Dir
                for _, child := range children {
                        err = RestoreAssets(dir, path.Join(name, child))
                        if err != nil {
                                return err
                        }
                }
        }
        return nil
}

func _filePath(dir, name string) string {
        cannonicalName := strings.Replace(name, "\\", "/", -1)
        return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}

