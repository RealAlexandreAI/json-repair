package jsonrepair

import (
	"strings"
	"testing"
)

func TestRepairJSONWithSchemaRepairsCommonHeadlessPayload(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"count": map[string]any{
				"type": "integer",
			},
			"enabled": map[string]any{
				"type":    "boolean",
				"default": true,
			},
			"tags": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required":             []any{"count", "enabled"},
		"additionalProperties": false,
	}

	got, err := RepairJSONWithSchema(`{"count":"42","tags":[1,"ok"],"extra":"drop"}`, schema)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"count":42,"enabled":true,"tags":["1","ok"]}`
	if !jsonStringsEqual(got, want) {
		t.Fatalf("RepairJSONWithSchema() = %s, want %s", got, want)
	}
}

func TestRepairJSONWithSchemaUnwrapsNestedJSONString(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"port": map[string]any{
						"type": "integer",
					},
				},
				"required": []any{"port"},
			},
		},
		"required": []any{"config"},
	}

	got, err := RepairJSONWithSchema(`{"config":"{\"port\":\"8080\"}"}`, schema)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"config":{"port":8080}}`
	if !jsonStringsEqual(got, want) {
		t.Fatalf("RepairJSONWithSchema() = %s, want %s", got, want)
	}
}

func TestRepairJSONWithSchemaRejectsMissingRequiredProperty(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type": "string",
			},
		},
		"required": []any{"name"},
	}

	_, err := RepairJSONWithSchema(`{"other":true}`, schema)
	if err == nil || !strings.Contains(err.Error(), "missing required property") {
		t.Fatalf("RepairJSONWithSchema() error = %v, want missing required property error", err)
	}
}

func TestRepairJSONWithSchemaSupportsReferencesEnumsAndDefaults(t *testing.T) {
	schema := map[string]any{
		"$defs": map[string]any{
			"port": map[string]any{
				"type": "integer",
			},
		},
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type":    "string",
				"enum":    []any{"fast", "safe"},
				"default": "safe",
			},
			"kind": map[string]any{
				"const":   "worker",
				"default": "worker",
			},
			"port": map[string]any{
				"$ref": "#/$defs/port",
			},
		},
		"required": []any{"port"},
	}

	got, err := RepairJSONWithSchema(`{"port":"9000","mode":"fast"}`, schema)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"kind":"worker","mode":"fast","port":9000}`
	if !jsonStringsEqual(got, want) {
		t.Fatalf("RepairJSONWithSchema() = %s, want %s", got, want)
	}

	_, err = RepairJSONWithSchema(`{"port":9000,"mode":"slow"}`, schema)
	if err == nil || !strings.Contains(err.Error(), "enum") {
		t.Fatalf("RepairJSONWithSchema() error = %v, want enum error", err)
	}
}

func TestRepairJSONWithSchemaRepairsAdditionalProperties(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"labels": map[string]any{
				"type": "object",
			},
		},
		"additionalProperties": map[string]any{
			"type": "integer",
		},
	}

	got, err := RepairJSONWithSchema(`{"labels":{"env":"prod"},"retries":"3"}`, schema)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"labels":{"env":"prod"},"retries":3}`
	if !jsonStringsEqual(got, want) {
		t.Fatalf("RepairJSONWithSchema() = %s, want %s", got, want)
	}
}

func TestRepairJSONWithSchemaRejectsUnmatchedAnyOf(t *testing.T) {
	schema := map[string]any{
		"anyOf": []any{
			map[string]any{"type": "integer"},
			map[string]any{"type": "boolean"},
		},
	}

	_, err := RepairJSONWithSchema(`"not a scalar"`, schema)
	if err == nil || !strings.Contains(err.Error(), "anyOf") {
		t.Fatalf("RepairJSONWithSchema() error = %v, want anyOf error", err)
	}
}

func TestRepairJSONWithSchemaPreservesEnumTypes(t *testing.T) {
	schema := map[string]any{
		"enum": []any{"1"},
	}

	_, err := RepairJSONWithSchema(`1`, schema)
	if err == nil || !strings.Contains(err.Error(), "enum") {
		t.Fatalf("RepairJSONWithSchema() error = %v, want enum type error", err)
	}
}

func TestRepairJSONWithSchemaAllowsMissingDisallowedOptionalProperty(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"forbidden": false,
		},
	}

	got, err := RepairJSONWithSchema(`{}`, schema)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(got, `{}`) {
		t.Fatalf("RepairJSONWithSchema() = %s, want {}", got)
	}
}

func TestRepairWithOptionsSchemaGuidesMissingValue(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{
				"type": "integer",
			},
		},
		"required": []any{"value"},
	}

	result, err := RepairWithOptions(`{"value": }`, RepairOptions{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `{"value":0}`) {
		t.Fatalf("RepairWithOptions() = %s, want schema-guided zero", result.JSON)
	}

	result, err = RepairWithOptions(`"1"`, RepairOptions{
		Schema:             map[string]any{"type": "integer"},
		SkipJSONValidation: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `1`) {
		t.Fatalf("RepairWithOptions() = %s, want schema-guided scalar", result.JSON)
	}
}

func TestRepairWithOptionsSchemaSalvageSelectsMatchingTopLevelFragment(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "integer"},
		},
		"required": []any{"name", "age"},
	}

	result, err := RepairWithOptions(`{"noise":true} {"name":"Alice","age":"30"}`, RepairOptions{
		Schema:           schema,
		SchemaRepairMode: SchemaRepairSalvage,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `{"age":30,"name":"Alice"}`) {
		t.Fatalf("RepairWithOptions() = %s, want matching fragment", result.JSON)
	}
}

func TestRepairWithOptionsSchemaSalvageMapsArrayToObject(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "integer"},
		},
		"required": []any{"name", "age"},
	}

	result, err := RepairWithOptions(`["30","Alice"]`, RepairOptions{
		Schema:           schema,
		SchemaRepairMode: SchemaRepairSalvage,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `{"age":30,"name":"Alice"}`) {
		t.Fatalf("RepairWithOptions() = %s, want mapped object", result.JSON)
	}
}

func TestRepairWithOptionsValidatesSchemaOptions(t *testing.T) {
	_, err := RepairWithOptions(`{}`, RepairOptions{SchemaRepairMode: SchemaRepairSalvage})
	if err == nil || !strings.Contains(err.Error(), "requires a schema") {
		t.Fatalf("RepairWithOptions() error = %v, want missing schema error", err)
	}

	_, err = RepairWithOptions(`{}`, RepairOptions{Schema: map[string]any{}, Strict: true})
	if err == nil || !strings.Contains(err.Error(), "schema and strict") {
		t.Fatalf("RepairWithOptions() error = %v, want schema/strict error", err)
	}

	_, err = RepairWithOptions(`{}`, RepairOptions{SchemaRepairMode: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "schema repair mode") {
		t.Fatalf("RepairWithOptions() error = %v, want mode error", err)
	}
}

func TestRepairWithOptionsSchemaLogsActions(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"count": map[string]any{
				"type": "integer",
			},
		},
		"additionalProperties": false,
	}

	result, err := RepairWithOptions(`{"count":"1","extra":true}`, RepairOptions{
		Schema:  schema,
		Logging: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Log) == 0 {
		t.Fatal("RepairWithOptions() returned no schema repair log")
	}
	for _, entry := range result.Log {
		if strings.Contains(entry.Text, "schema") {
			return
		}
	}
	t.Fatalf("RepairWithOptions() log = %#v, want schema action", result.Log)
}

func TestRepairWithOptionsSchemaSupportsPatternProperties(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"patternProperties": map[string]any{
			"^x_": map[string]any{"type": "integer"},
		},
		"additionalProperties": false,
	}

	result, err := RepairWithOptions(`{"x_count":"2","other":true}`, RepairOptions{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `{"x_count":2}`) {
		t.Fatalf("RepairWithOptions() = %s, want pattern property repair", result.JSON)
	}
}

func TestRepairWithOptionsSchemaSupportsTupleItems(t *testing.T) {
	schema := map[string]any{
		"type": "array",
		"items": []any{
			map[string]any{"type": "integer"},
			map[string]any{"type": "string"},
		},
		"additionalItems": false,
	}

	result, err := RepairWithOptions(`["1",2,true]`, RepairOptions{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `[1,"2"]`) {
		t.Fatalf("RepairWithOptions() = %s, want tuple repair", result.JSON)
	}
}

func TestRepairWithOptionsSchemaValidatesCommonConstraints(t *testing.T) {
	stringSchema := map[string]any{
		"type":      "string",
		"minLength": 3,
		"pattern":   "^[a-z]+$",
	}
	_, err := RepairWithOptions(`"AB"`, RepairOptions{Schema: stringSchema})
	if err == nil || !strings.Contains(err.Error(), "minLength") {
		t.Fatalf("RepairWithOptions() error = %v, want minLength error", err)
	}

	numberSchema := map[string]any{
		"type":    "number",
		"minimum": 10,
		"maximum": 20,
	}
	_, err = RepairWithOptions(`25`, RepairOptions{Schema: numberSchema})
	if err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Fatalf("RepairWithOptions() error = %v, want maximum error", err)
	}

	uniqueSchema := map[string]any{
		"type":        "array",
		"uniqueItems": true,
	}
	_, err = RepairWithOptions(`[1,1]`, RepairOptions{Schema: uniqueSchema})
	if err == nil || !strings.Contains(err.Error(), "uniqueItems") {
		t.Fatalf("RepairWithOptions() error = %v, want uniqueItems error", err)
	}
}
