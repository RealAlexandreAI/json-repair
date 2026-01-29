package jsonrepair

import (
	"testing"
)

func TestNewline19(t *testing.T) {
	input := `{"a": "", "b": ""}
garbage`
	result, err := RepairJSON(input)
	t.Log("Input:", input)
	t.Log("Output:", result)
	t.Log("Error:", err)
	
	want := `{"a":"","b":""}`
	if !jsonStringsEqual(result, want) {
		t.Errorf("Got %v, want %v", result, want)
	}
}

func TestNewline19B(t *testing.T) {
	input := `{
  "Be": "",
  "gone": ""
}
",п"г`
	result, err := RepairJSON(input)
	t.Log("Input:", input)
	t.Log("Output:", result)
	t.Log("Error:", err)
	
	want := `{"Be":"","gone":""}`
	if !jsonStringsEqual(result, want) {
		t.Errorf("Got %v, want %v", result, want)
	}
}
