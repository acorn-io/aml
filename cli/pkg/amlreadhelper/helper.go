package amlreadhelper

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/acorn-io/aml"
	"sigs.k8s.io/yaml"
)

func ReadFile(name string) ([]byte, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	if isYAMLFilename(name) {
		data := map[string]any{}
		if err := UnmarshalFile(name, data); err != nil {
			return nil, err
		}
		return json.Marshal(data)
	}

	return data, nil
}

func UnmarshalFile(name string, out any) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	if isYAMLFilename(name) {
		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		return yaml.Unmarshal(data, out)
	}

	return aml.NewDecoder(f).Decode(out)
}

func isYAMLFilename(v string) bool {
	for _, suffix := range []string{".yaml", ".yml"} {
		if strings.HasSuffix(strings.ToLower(v), suffix) {
			return true
		}
	}
	return false
}
