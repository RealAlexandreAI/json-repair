package jsonrepair

import (
	"testing"
)

func TestSimplified19(t *testing.T) {
	input := `{"a": "", "b": ""}garbage`
	result, err := RepairJSON(input)
	t.Log("Input:", input)
	t.Log("Output:", result)
	t.Log("Error:", err)
	
	want := `{"a":"","b":""}`
	if !jsonStringsEqual(result, want) {
		t.Errorf("Got %v, want %v", result, want)
	}
}
