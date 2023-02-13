package loader

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/acorn-io/aml/pkg/amlparser"
	"github.com/acorn-io/aml/pkg/cue"
	"github.com/acorn-io/aml/pkg/definition"
)

func CreateReader(path string) (io.ReadCloser, error) {
	s, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if s.IsDir() {
		return readDir(path)
	}
	return os.Open(path)
}

func readDir(path string) (io.ReadCloser, error) {
	buffer := &bytes.Buffer{}
	tarWriter := tar.NewWriter(buffer)
	root := os.DirFS(path)
	err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		fi, err := fs.Stat(root, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := root.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tarWriter, f)
		_ = f.Close()
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tarWriter.Close(); err != nil {
		return nil, err
	}
	return io.NopCloser(buffer), nil
}

func ToFiles(r io.Reader) (result []cue.File, _ error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// tar files must be at least 1k long
	if len(data) < 1024 {
		return definition.NewAcornfile(data), nil
	}

	tarReader := tar.NewReader(bytes.NewBuffer(data))
	header, err := tarReader.Next()
	if errors.Is(err, tar.ErrHeader) {
		return definition.NewAcornfile(data), nil
	} else if err != nil {
		return nil, err
	}

	files := map[string]interface{}{}
	for {
		if header.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(strings.ToLower(header.Name), ".aml") {
			result = append(result, cue.File{
				Filename:    header.Name[:len(header.Name)-3] + ".cue",
				DisplayName: header.Name,
				Data:        data,
				Parser:      amlparser.ParseFile,
			})
		} else if !utf8.Valid(content) {
			return nil, fmt.Errorf("Invalid utf-8 content in [%s]", header.Name)
		} else {
			addFile(files, header.Name, string(content))
		}

		header, err = tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}

	if len(files) > 0 {
		filesFile, err := toFiles(files)
		if err != nil {
			return nil, err
		}
		result = append(result, filesFile)
	}

	return result, nil
}

func toFiles(files map[string]any) (cue.File, error) {
	data, err := json.Marshal(map[string]any{
		"std": map[string]any{
			"files": files,
		},
	})
	if err != nil {
		return cue.File{}, err
	}
	return cue.File{
		Filename: "files.cue",
		Data:     data,
	}, nil
}

func addFile(files map[string]any, filename, content string) {
	parts := strings.Split(filename, "/")

	for i, part := range parts {
		if i == len(parts)-1 {
			files[part] = content
		} else {
			sub, ok := files[part].(map[string]any)
			if !ok {
				sub = map[string]any{}
				files[part] = sub
			}
			files = sub
		}
	}
}
