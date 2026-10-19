package jira

import (
	"encoding/json"
	"strings"
	"testing"
)

// walkNodes visits every object in a marshalled ADF tree that has a "type".
func walkNodes(node interface{}, fn func(typeName string, obj map[string]interface{})) {
	switch v := node.(type) {
	case map[string]interface{}:
		if tn, ok := v["type"].(string); ok {
			fn(tn, v)
		}
		if content, ok := v["content"].([]interface{}); ok {
			for _, child := range content {
				walkNodes(child, fn)
			}
		}
	case []interface{}:
		for _, child := range v {
			walkNodes(child, fn)
		}
	}
}

// Container nodes must never serialize a "text" property: Jira rejects the
// whole document with "not valid Atlassian Document Format (ADF) content".
func TestTextToADFProducesSchemaCleanNodes(t *testing.T) {
	doc := TextToADF("hello world\n- list item\n# heading\n```\ncode\n```")
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	containers := map[string]bool{
		"doc": true, "paragraph": true, "bulletList": true,
		"listItem": true, "heading": true, "codeBlock": true, "blockquote": true,
	}
	walkNodes(root, func(typeName string, obj map[string]interface{}) {
		if _, hasText := obj["text"]; hasText && containers[typeName] {
			t.Errorf("%s node carries a stray \"text\" property (%v): %s", typeName, obj["text"], b)
		}
	})
}

func TestTextToADFHeadingLevelIsInteger(t *testing.T) {
	b, _ := json.Marshal(TextToADF("# Title"))
	s := string(b)
	if !strings.Contains(s, `"level":2`) {
		t.Fatalf("heading attrs must be {\"level\":2}, got: %s", s)
	}
	if strings.Contains(s, `"attrs":{"text"`) {
		t.Fatalf("heading attrs must not use text, got: %s", s)
	}
}

// A mention notifies only when attrs.id carries the accountId, and it is a
// leaf node: no child content, no stray empty fields.
func TestMentionNodeCarriesAccountID(t *testing.T) {
	b, _ := json.Marshal(ADFNode{Type: "mention", Attrs: &ADFAttrs{ID: "abc123", Text: "René"}})
	want := `{"type":"mention","attrs":{"id":"abc123","text":"René"}}`
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
}

func TestTextToADFRoundTripsThroughFlatten(t *testing.T) {
	doc := TextToADF("line one\n- bullet\n# heading")
	got := doc.Flatten()
	for _, want := range []string{"line one", "- bullet", "## heading"} {
		if !strings.Contains(got, want) {
			t.Errorf("Flatten() = %q, missing %q", got, want)
		}
	}
}
