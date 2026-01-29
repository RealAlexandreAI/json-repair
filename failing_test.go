package jsonrepair

import "testing"

func TestCase40_Debug(t *testing.T) {
	in := `{"value_1": true, SHOULD_NOT_EXIST "value_2": "data" AAAA }`
	want := `{"value_1":true,"value_2":"data"}`
	got, _ := RepairJSON(in)
	t.Logf("Input: %s", in)
	t.Logf("Got: %s", got)
	t.Logf("Want: %s", want)
	if !jsonStringsEqual(got, want) {
		t.Error("Mismatch")
	}
}

func TestCase51_Debug(t *testing.T) {
	in := `{""answer"":[{""traits"":''Female aged 60+'',""answer1"":""5""}]}`
	want := `{"answer":[{"traits":"Female aged 60+","answer1":"5"}]}`
	got, _ := RepairJSON(in)
	t.Logf("Input: %s", in)
	t.Logf("Got: %s", got)
	t.Logf("Want: %s", want)
	if !jsonStringsEqual(got, want) {
		t.Error("Mismatch")
	}
}
