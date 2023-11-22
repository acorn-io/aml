package aml

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/acorn-io/aml/pkg/parser/filemap"
	"gopkg.in/yaml.v3"
)

func yamlToJSON(name string) ([]byte, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func Open(name string) (io.ReadCloser, error) {
	if isYAMLFilename(name) {
		data, err := yamlToJSON(name)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewBuffer(data)), nil
	}

	fi, err := os.Stat(name)
	if err == nil && fi.IsDir() {
		fm, err := filemap.FromDirectory(name)
		if err != nil {
			return nil, err
		}
		r, err := fm.ToReader()
		return io.NopCloser(r), err
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	dotD := name + ".d"
	fi, err = os.Stat(dotD)
	if err == nil && fi.IsDir() {
		defer f.Close()

		fm, err := filemap.FromDirectory(dotD)
		if err != nil {
			return nil, err
		}

		data, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		fm.AddFile(name, data)

		r, err := fm.ToReader()
		return io.NopCloser(r), err
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return f, nil
}

func ReadFile(name string) ([]byte, error) {
	f, err := Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

func UnmarshalFile(name string, out any) error {
	f, err := Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	return NewDecoder(f).Decode(out)
}

func isYAMLFilename(v string) bool {
	for _, suffix := range []string{".yaml", ".yml"} {
		if strings.HasSuffix(strings.ToLower(v), suffix) {
			return true
		}
	}
	return false
}
