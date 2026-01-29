package jsonrepair

import "testing"

func TestCase43(t *testing.T) {
	in := `{ "content": "[LINK]("https://google.com")" }`
	want := `{"content":"[LINK](\"https://google.com\")"}`
	got, _ := RepairJSON(in)
	t.Log("Input:", in)
	t.Log("Got:", got)
	t.Log("Want:", want)
	if !jsonStringsEqual(got, want) {
		t.Errorf("Mismatch")
	}
}

func TestCase51(t *testing.T) {
	in := `{""answer"":[{""traits"":''Female aged 60+'',""answer1"":""5""}]}`
	want := `{"answer":[{"traits":"Female aged 60+","answer1":"5"}]}`
	got, _ := RepairJSON(in)
	t.Log("Input:", in)
	t.Log("Got:", got)
	t.Log("Want:", want)
	if !jsonStringsEqual(got, want) {
		t.Errorf("Mismatch")
	}
}
