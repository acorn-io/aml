package filemap

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
)

var Header = []byte("//aml:filemap")

type FileMap struct {
	files map[string][]byte
}

type Entry struct {
	Filename string
	Data     []byte
}

func (f *FileMap) Files() (result []Entry) {
	filenames := maps.Keys(f.files)
	sort.Strings(filenames)

	for _, filename := range filenames {
		result = append(result, Entry{
			Filename: filename,
			Data:     f.files[filename],
		})
	}

	return
}

func (f *FileMap) MarshalJSON() ([]byte, error) {
	out := map[string]string{}
	for k, v := range f.files {
		out[k] = string(v)
	}
	return json.Marshal(out)
}

func (f *FileMap) ToReader() (io.Reader, error) {
	out := &bytes.Buffer{}
	out.Write(Header)
	out.WriteByte('\n')

	return out, json.NewEncoder(out).Encode(f)
}

func forSingleFile(filename string, data []byte) *FileMap {
	return &FileMap{
		files: map[string][]byte{
			filename: data,
		},
	}
}

func FromBytes(filename string, data []byte) (*FileMap, error) {
	if !bytes.HasPrefix(data, Header) {
		return forSingleFile(filename, data), nil
	}

	i := bytes.IndexByte(data, '{')
	if i < 0 {
		return forSingleFile(filename, data), nil
	}

	files := map[string]string{}
	if err := json.Unmarshal(data[i:], &files); err != nil {
		return nil, err
	}

	result := &FileMap{
		files: map[string][]byte{},
	}
	for k, v := range files {
		if filename == "" {
			result.files[k] = []byte(v)
		} else {
			result.files[filepath.Join(filename, k)] = []byte(v)
		}
	}
	return result, nil
}

func isValidExt(name string) bool {
	if name == "Acornfile" {
		return true
	}
	ext := filepath.Ext(name)
	return strings.EqualFold(ext, ".aml") || strings.EqualFold(ext, ".acorn")
}

func FromDirectory(dir string) (*FileMap, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	result := &FileMap{
		files: map[string][]byte{},
	}

	for _, entry := range entries {
		if entry.IsDir() || !isValidExt(entry.Name()) {
			continue
		}
		filename := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		result.files[entry.Name()] = data
	}

	return result, nil
}
