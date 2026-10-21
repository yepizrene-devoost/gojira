package mcpserver

func updateFieldsSchema() map[string]any {
	fields := objectSchema(map[string]any{
		"summary":          map[string]any{"type": "string"},
		"description":      map[string]any{"type": "string", "description": "Plain text converted to ADF; empty string clears description. Replacing a description this way may discard rich ADF formatting that plain text cannot represent"},
		"priority":         map[string]any{"type": "string"},
		"labels":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
		"dueDate":          map[string]any{"type": "string", "description": "Real calendar date YYYY-MM-DD; empty string clears dueDate."},
		"components":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Component names; [] clears components."},
		"fixVersions":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Fix version names; [] clears fixVersions."},
		"storyPoints":      map[string]any{"type": []string{"number", "null"}, "description": "Finite decimal number; null clears storyPoints. Requires storyPointsField."},
		"storyPointsField": map[string]any{"type": "string"},
	})
	fields["minProperties"] = 1
	return fields
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
