package jsonrepair

import (
	"os"
	"strings"
	"testing"
)

func TestRepairValueReturnsParsedValue(t *testing.T) {
	value, err := RepairValue(`{"name":"Ada","age":}`)
	if err != nil {
		t.Fatal(err)
	}

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("RepairValue() type = %T, want map[string]any", value)
	}
	if object["name"] != "Ada" || object["age"] != "" {
		t.Fatalf("RepairValue() = %#v, want repaired object", object)
	}
}

func TestRepairWithOptionsReturnsRepairLog(t *testing.T) {
	result, err := RepairWithOptions(`{"key":value}`, RepairOptions{Logging: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Log) == 0 {
		t.Fatal("RepairWithOptions() returned no repair log")
	}
	if result.Value == nil || result.JSON == "" {
		t.Fatalf("RepairWithOptions() = %#v, want value and JSON", result)
	}
	if result.Log[0].Context == "" || result.Log[0].Text == "" {
		t.Fatalf("RepairWithOptions() log = %#v, want text and context", result.Log[0])
	}
}

func TestRepairWithOptionsStrictRejectsRepairableStructure(t *testing.T) {
	cases := []struct {
		name  string
		input string
		match string
	}{
		{name: "duplicate key", input: `{"key":"first","key":"second"}`, match: "duplicate key"},
		{name: "missing colon", input: `{"key" "value"}`, match: "missing ':'"},
		{name: "empty key", input: `{ "": "value" }`},
		{name: "empty value", input: `{"key": , "other":"value"}`, match: "parsed value is empty"},
		{name: "multiple top level", input: `{"key":"value"}{"other":"value"}`, match: "multiple top-level"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := RepairWithOptions(test.input, RepairOptions{Strict: true})
			if err == nil || (test.match != "" && !strings.Contains(err.Error(), test.match)) {
				t.Fatalf("RepairWithOptions() error = %v, want %s error", err, test.match)
			}
		})
	}
}

func TestRepairWithOptionsStreamStableKeepsPartialString(t *testing.T) {
	input := `{"key":"value\n123,` + "`key2:value2" + ``
	unstable, err := RepairWithOptions(input, RepairOptions{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := RepairWithOptions(input, RepairOptions{StreamStable: true})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `{"key":"value\n123,`+"`key2:value2"+`"}`) {
		t.Fatalf("RepairWithOptions() = %s, want stable partial string", result.JSON)
	}
	if jsonStringsEqual(unstable.JSON, result.JSON) {
		t.Fatalf("default repair = %s, want different unstable result", unstable.JSON)
	}

	result, err = RepairWithOptions(`{"key":"value\`, RepairOptions{StreamStable: true})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `{"key":"value"}`) {
		t.Fatalf("RepairWithOptions() = %s, want stable trailing escape", result.JSON)
	}
}

func TestRepairWithOptionsLoggingValidJSONReturnsEmptyLog(t *testing.T) {
	result, err := RepairWithOptions(`{"key":"value"}`, RepairOptions{Logging: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Log == nil || len(result.Log) != 0 {
		t.Fatalf("RepairWithOptions() log = %#v, want empty non-nil log", result.Log)
	}
}

func TestRepairReaderAndFile(t *testing.T) {
	input := `{"value": }`
	readerResult, err := RepairReader(strings.NewReader(input), RepairOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(readerResult.JSON, `{"value":""}`) {
		t.Fatalf("RepairReader() = %s, want repaired value", readerResult.JSON)
	}

	path := t.TempDir() + "/input.json"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	fileResult, err := RepairFile(path, RepairOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(fileResult.JSON, `{"value":""}`) {
		t.Fatalf("RepairFile() = %s, want repaired value", fileResult.JSON)
	}
}

func TestRepairWithOptionsRejectsExcessiveNesting(t *testing.T) {
	input := strings.Repeat("{value:", maxRecursionDepth+1) + "0" + strings.Repeat("}", maxRecursionDepth+1)
	_, err := RepairWithOptions(input, RepairOptions{})
	if err == nil || !strings.Contains(err.Error(), "recursion depth") {
		t.Fatalf("RepairWithOptions() error = %v, want recursion depth error", err)
	}
}

func TestRepairWithOptionsSchemaSalvageDropsInvalidArrayItems(t *testing.T) {
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "integer",
		},
	}

	result, err := RepairWithOptions(`[1,"bad",2]`, RepairOptions{
		Schema:           schema,
		SchemaRepairMode: SchemaRepairSalvage,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonStringsEqual(result.JSON, `[1,2]`) {
		t.Fatalf("RepairWithOptions() = %s, want salvaged array", result.JSON)
	}

	_, err = RepairWithOptions(`[1,"bad",2]`, RepairOptions{Schema: schema})
	if err == nil {
		t.Fatal("RepairWithOptions() accepted invalid array item in standard mode")
	}
}
