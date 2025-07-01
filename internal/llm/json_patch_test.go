package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONPatcher_ApplyPatch(t *testing.T) {
	patcher := NewJSONPatcher()

	tests := []struct {
		name      string
		input     string
		patch     JSONPatch
		expected  string
		expectErr bool
	}{
		{
			name:  "simple field update",
			input: `{"name": "old", "value": 123}`,
			patch: JSONPatch{
				Path:  "name",
				Value: "new",
			},
			expected: `{
  "name": "new",
  "value": 123
}`,
		},
		{
			name:  "nested field update",
			input: `{"user": {"name": "old", "age": 30}}`,
			patch: JSONPatch{
				Path:  "user.name",
				Value: "new",
			},
			expected: `{
  "user": {
    "age": 30,
    "name": "new"
  }
}`,
		},
		{
			name:  "array element update",
			input: `{"items": ["a", "b", "c"]}`,
			patch: JSONPatch{
				Path:  "items[1]",
				Value: "updated",
			},
			expected: `{
  "items": [
    "a",
    "updated",
    "c"
  ]
}`,
		},
		{
			name:  "nested array update",
			input: `{"data": {"items": [{"id": 1}, {"id": 2}]}}`,
			patch: JSONPatch{
				Path:  "data.items[0].id",
				Value: 100,
			},
			expected: `{
  "data": {
    "items": [
      {
        "id": 100
      },
      {
        "id": 2
      }
    ]
  }
}`,
		},
		{
			name:  "add new field",
			input: `{"existing": "value"}`,
			patch: JSONPatch{
				Path:  "new",
				Value: "field",
			},
			expected: `{
  "existing": "value",
  "new": "field"
}`,
		},
		{
			name:  "array out of bounds",
			input: `{"items": ["a", "b"]}`,
			patch: JSONPatch{
				Path:  "items[5]",
				Value: "invalid",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := patcher.ApplyPatch(json.RawMessage(tt.input), tt.patch)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(result))
		})
	}
}

func TestJSONPatcher_ApplyPatches(t *testing.T) {
	patcher := NewJSONPatcher()

	input := `{
		"issues": [
			{
				"id": "issue_001",
				"transaction_ids": null,
				"affected_count": 5
			}
		],
		"coherence_score": 0.85
	}`

	patches := []JSONPatch{
		{
			Path:  "issues[0].transaction_ids",
			Value: []string{"txn1", "txn2", "txn3", "txn4", "txn5"},
		},
		{
			Path:  "coherence_score",
			Value: 0.92,
		},
	}

	result, err := patcher.ApplyPatches(json.RawMessage(input), patches)
	require.NoError(t, err)

	expected := `{
		"issues": [
			{
				"id": "issue_001",
				"transaction_ids": ["txn1", "txn2", "txn3", "txn4", "txn5"],
				"affected_count": 5
			}
		],
		"coherence_score": 0.92
	}`

	assert.JSONEq(t, expected, string(result))
}

func TestJSONPatcher_ParsePath(t *testing.T) {
	patcher := NewJSONPatcher()

	tests := []struct {
		path     string
		expected []string
	}{
		{
			path:     "simple",
			expected: []string{"simple"},
		},
		{
			path:     "nested.path",
			expected: []string{"nested", "path"},
		},
		{
			path:     "array[0]",
			expected: []string{"array", "0"},
		},
		{
			path:     "complex.array[10].field",
			expected: []string{"complex", "array", "10", "field"},
		},
		{
			path:     "multiple[0][1].deep",
			expected: []string{"multiple", "0", "1", "deep"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := patcher.parsePath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
