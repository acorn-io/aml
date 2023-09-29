package eval

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acorn-io/aml/pkg/parser"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	dir := fmt.Sprintf("testdata/%s", t.Name())
	files, err := os.ReadDir(dir)
	require.Nil(t, err)

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".acorn") {
			continue
		}
		t.Run(strings.TrimSuffix(file.Name(), ".acorn"), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, file.Name()))
			require.NoError(t, err)

			ast, err := parser.ParseFile(file.Name(), bytes.NewReader(data))
			if err != nil {
				autogold.ExpectFile(t, err)
				return
			}

			result, err := Build(ast)
			if err != nil {
				autogold.ExpectFile(t, err)
			} else {
				autogold.ExpectFile(t, result)
			}
		})
	}
}
