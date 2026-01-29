package jsonrepair

import (
	"testing"
)

func TestIssue19(t *testing.T) {
	input := `
    {
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

func TestIssue18(t *testing.T) {
	input := `{"name": "John is "good",hah", "age": 30}`
	
	result, err := RepairJSON(input)
	t.Log("Input:", input)
	t.Log("Output:", result)
	t.Log("Error:", err)
	
	want := `{"name":"John is \"good\",hah","age":30}`
	if !jsonStringsEqual(result, want) {
		t.Errorf("Got %v, want %v", result, want)
	}
}
