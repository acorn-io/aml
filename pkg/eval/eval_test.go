package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acorn-io/aml/pkg/parser"
	"github.com/acorn-io/aml/pkg/value"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEval(t *testing.T) {
	ctx := WithScope(context.Background(), Builtin)
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
			require.NoError(t, err)

			result, err := Build(ast)
			require.NoError(t, err)

			v, ok, err := result.ToValue(ctx)
			if err == nil {
				assert.True(t, ok)

				var data []byte
				nv, ok, err := value.NativeValue(v)
				if err == nil {
					require.True(t, ok)
					data, err = json.MarshalIndent(nv, "", "  ")
					require.NoError(t, err)
					autogold.ExpectFile(t, autogold.Raw(data))
				} else {
					autogold.ExpectFile(t, autogold.Raw(err.Error()))
				}
			} else {
				autogold.ExpectFile(t, err.Error())
			}
		})
	}
}

func TestSchemaRender(t *testing.T) {
	ctx := WithScope(WithSchema(context.Background(), true), Builtin)
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
			require.NoError(t, err)

			result, err := Build(ast)
			require.NoError(t, err)

			v, _, err := result.ToValue(WithSchema(ctx, true))
			if err == nil {
				summary := value.Summarize(v.(*value.TypeSchema))
				data, err := json.MarshalIndent(summary, "", "  ")
				require.NoError(t, err)
				autogold.ExpectFile(t, autogold.Raw(data))
			} else {
				autogold.ExpectFile(t, err)
			}
		})
	}
}

func TestFileDescribe(t *testing.T) {
	ctx := WithScope(context.Background(), Builtin)
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
			require.NoError(t, err)

			result, err := Build(ast)
			require.NoError(t, err)

			file, err := result.Describe(ctx)
			if err == nil {
				data, err := json.MarshalIndent(file, "", "  ")
				require.NoError(t, err)
				autogold.ExpectFile(t, autogold.Raw(data))
			} else {
				autogold.ExpectFile(t, err)
			}
		})
	}
}
