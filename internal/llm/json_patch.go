package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// JSONPatcher applies patches to JSON data.
type JSONPatcher struct{}

// NewJSONPatcher creates a new JSON patcher.
func NewJSONPatcher() *JSONPatcher {
	return &JSONPatcher{}
}

// ApplyPatch applies a single patch to JSON data.
func (p *JSONPatcher) ApplyPatch(data json.RawMessage, patch JSONPatch) (json.RawMessage, error) {
	// Parse the JSON into a generic structure
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Apply the patch
	if err := p.setPatchValue(&root, patch.Path, patch.Value); err != nil {
		return nil, fmt.Errorf("failed to apply patch at path %s: %w", patch.Path, err)
	}

	// Marshal back to JSON
	result, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patched data: %w", err)
	}

	return result, nil
}

// ApplyPatches applies multiple patches to JSON data.
func (p *JSONPatcher) ApplyPatches(data json.RawMessage, patches []JSONPatch) (json.RawMessage, error) {
	result := data
	for i, patch := range patches {
		var err error
		result, err = p.ApplyPatch(result, patch)
		if err != nil {
			return nil, fmt.Errorf("failed to apply patch %d: %w", i, err)
		}
	}
	return result, nil
}

// setPatchValue sets a value at the given JSON path.
func (p *JSONPatcher) setPatchValue(data *interface{}, path string, value interface{}) error {
	segments := p.parsePath(path)
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// Navigate to the parent of the target
	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]
		next, err := p.navigate(current, segment)
		if err != nil {
			return fmt.Errorf("failed to navigate to %s: %w", segment, err)
		}
		current = next
	}

	// Set the value at the final segment
	lastSegment := segments[len(segments)-1]
	return p.setValue(current, lastSegment, value)
}

// parsePath parses a JSON path into segments.
// Supports:
// - Simple keys: "foo"
// - Nested keys: "foo.bar"
// - Array indices: "foo[0]"
// - Complex paths: "foo.bar[0].baz".
func (p *JSONPatcher) parsePath(path string) []string {
	var segments []string

	// Handle array notation by converting foo[0] to foo.0
	arrayPattern := regexp.MustCompile(`\[(\d+)\]`)
	path = arrayPattern.ReplaceAllString(path, ".$1")

	// Split by dots
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}

	return segments
}

// navigate moves to the next level in the data structure.
func (p *JSONPatcher) navigate(data *interface{}, segment string) (*interface{}, error) {
	switch v := (*data).(type) {
	case map[string]interface{}:
		// Object navigation
		if val, ok := v[segment]; ok {
			return &val, nil
		}
		// Create the key if it doesn't exist
		v[segment] = make(map[string]interface{})
		val := v[segment]
		return &val, nil

	case []interface{}:
		// Array navigation
		index, err := strconv.Atoi(segment)
		if err != nil {
			return nil, fmt.Errorf("invalid array index %s", segment)
		}
		if index < 0 || index >= len(v) {
			return nil, fmt.Errorf("array index %d out of bounds (len=%d)", index, len(v))
		}
		return &v[index], nil

	default:
		return nil, fmt.Errorf("cannot navigate into %T with segment %s", v, segment)
	}
}

// setValue sets the value at the final segment.
func (p *JSONPatcher) setValue(data *interface{}, segment string, value interface{}) error {
	switch v := (*data).(type) {
	case map[string]interface{}:
		// Setting object property
		v[segment] = value
		return nil

	case []interface{}:
		// Setting array element
		index, err := strconv.Atoi(segment)
		if err != nil {
			return fmt.Errorf("invalid array index %s", segment)
		}
		if index < 0 || index >= len(v) {
			return fmt.Errorf("array index %d out of bounds (len=%d)", index, len(v))
		}
		v[index] = value
		return nil

	case *interface{}:
		// If we have a pointer to interface, try to dereference
		if v != nil && *v != nil {
			return p.setValue(v, segment, value)
		}
		// Create a new map if nil
		newMap := make(map[string]interface{})
		newMap[segment] = value
		*data = newMap
		return nil

	default:
		return fmt.Errorf("cannot set value on %T", v)
	}
}

// ExtractValue extracts a value from JSON data at the given path.
func (p *JSONPatcher) ExtractValue(data json.RawMessage, path string) (interface{}, error) {
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	segments := p.parsePath(path)
	current := &root

	for _, segment := range segments {
		next, err := p.navigate(current, segment)
		if err != nil {
			return nil, fmt.Errorf("failed to navigate to %s: %w", segment, err)
		}
		current = next
	}

	return *current, nil
}
