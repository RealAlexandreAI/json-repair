package jsonrepair

import "testing"

func TestSimpleUnescaped(t *testing.T) {
	inputs := []struct{
		in string
		want string
	}{
		// Simplest case
		{`{"a": "x"y"}`, `{"a":"x\"y"}`},
		// With letter after quote  
		{`{"a": "x"y", "b": 1}`, `{"a":"x\"y","b":1}`},
		// Issue 18 original
		{`{"name": "John is "good",hah", "age": 30}`, `{"name":"John is \"good\",hah","age":30}`},
	}
	
	for i, tc := range inputs {
		got, _ := RepairJSON(tc.in)
		t.Logf("Test %d:", i)
		t.Logf("  In:   %s", tc.in)
		t.Logf("  Got:  %s", got)
		t.Logf("  Want: %s", tc.want)
		if !jsonStringsEqual(got, tc.want) {
			t.Errorf("Test %d failed", i)
		}
	}
}
