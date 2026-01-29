package jsonrepair

import (
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
)

// Test_RepairJSON
//
//	Description:
//	param t
func Test_RepairJSON(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{
			in: `
				{
					"name": "John",
					"age": 30,
					"isMarried": false
				}`,
			want: `{"name":"John","age":30,"isMarried":false}`,
		},
		{
			in: "```json\n" +
				"{\n" +
				"	\"name\": \"John\",\n" +
				"	\"age\": 30,\n" +
				"	\"isMarried\": false\n" +
				"}\n" +
				"```",
			want: `{"age":30,"isMarried":false,"name":"John"}`,
		},
		{
			in:   "[]",
			want: `[]`,
		},
		{
			in:   "   {  }   ",
			want: `{}`,
		},
		{
			in:   `"`,
			want: `""`,
		},
		{
			in:   "\n",
			want: `""`,
		},
		{
			in:   `  {"key": true, "key2": false, "key3": null}`,
			want: `{"key":true,"key2":false,"key3":null}`,
		},
		{
			in:   "{\"key\": TRUE, \"key2\": FALSE, \"key3\": Null } ",
			want: `{"key":true,"key2":false,"key3":null}`,
		},
		{
			in:   "{\"key\": TRUE, \"key2\": FALSE, \"key3\": Null  ",
			want: `{"key":true,"key2":false,"key3":null}`,
		},
		{
			in:   "{'key': 'string', 'key2': false, \"key3\": null, \"key4\": unquoted}",
			want: `{"key":"string","key2":false,"key3":null,"key4":"unquoted"}`,
		},
		{
			in:   `{"name": "John", "age": 30, "city": "New York"}`,
			want: `{"name":"John","age":30,"city":"New York"}`,
		},
		{
			in:   "[1, 2, 3, 4]",
			want: `[1,2,3,4]`,
		},
		{
			in:   "[1, 2, 3, 4",
			want: `[1,2,3,4]`,
		},
		{
			in:   `{"employees":["John", "Anna", "Peter"]} `,
			want: `{"employees":["John","Anna","Peter"]}`,
		},

		{
			in:   `{"name": "John", "age": 30, "city": "New York`,
			want: `{"age":30,"city":"New York","name":"John"}`,
		},
		{
			in:   `{"name": "John", "age": 30, city: "New York"}`,
			want: `{"age":30,"city":"New York","name":"John"}`,
		},
		{
			in:   `{"name": "John", "age": 30, "city": New York}`,
			want: `{"age":30,"city":"New York","name":"John"}`,
		},
		{
			in:   `{"name": John, "age": 30, "city": "New York"}`,
			want: `{"age":30,"city":"New York","name":"John"}`,
		},
		{
			in:   `[1, 2, 3,`,
			want: `[1,2,3]`,
		},
		{
			in:   `{"employees":["John", "Anna",`,
			want: `{"employees":["John","Anna"]}`,
		},

		{
			in:   " ",
			want: `""`,
		},
		{
			in:   "[",
			want: "[]",
		},
		{
			in:   "]",
			want: `""`,
		},
		{
			in:   "[[1\n\n]",
			want: "[[1]]",
		},
		{
			in:   "{",
			want: "{}",
		},
		{
			in:   "}",
			want: `""`,
		},
		{
			in:   `{"`,
			want: `{}`,
		},
		{
			in:   `["`,
			want: `[]`,
		},
		{
			in:   `'\"'`,
			want: `""`,
		},
		{
			in:   "string",
			want: `""`,
		},
		{
			in:   `{foo: [}`,
			want: `{"foo":[]}`,
		},
		{
			in:   `{"key": "value:value"}`,
			want: `{"key":"value:value"}`,
		},
		// TODO Full-width character support
		//{
		//	in:       `{“slanted_delimiter”: "value"}`,
		//	want: `{"slanted_delimiter": "value"}`,
		//},

		{
			in:   `{"name": "John", "age": 30, "city": "New`,
			want: `{"age":30,"city":"New","name":"John"}`,
		},
		{
			in:   `{"employees":["John", "Anna", "Peter`,
			want: `{"employees":["John","Anna","Peter"]}`,
		},
		{
			in:   `{"employees":["John", "Anna", "Peter"]}`,
			want: `{"employees":["John","Anna","Peter"]}`,
		},
		{
			in:   `{"text": "The quick brown fox,"}`,
			want: `{"text":"The quick brown fox,"}`,
		},
		{
			in:   `{"text": "The quick brown fox won\'t jump"}`,
			want: `{"text":"The quick brown fox won't jump"}`,
		},

		{
			in:   `{"value_1": "value_2": "data"}`,
			want: `{"value_1":"value_2\": \"data"}`,
		},
		{
			in:   `{"value_1": true, COMMENT "value_2": "data"}`,
			want: `{"value_1":true,"value_2":"data"}`,
		},
		{
			in:   `{"value_1": true, SHOULD_NOT_EXIST "value_2": "data" AAAA }`,
			want: `{"value_1":true,"value_2":"data"}`,
		},
		{
			in:   `{"": true, "key2": "value2"}`,
			want: `{"":true,"key2":"value2"}`,
		},
		{
			in:   ` - { "test_key": ["test_value", "test_value2"] }`,
			want: `{"test_key":["test_value","test_value2"]}`,
		},
		{
			in:   `{ "content": "[LINK]("https://google.com")" }`,
			want: `{"content":"[LINK](\"https://google.com\")"}`,
		},
		{
			in:   `{ "content": "[LINK](" }`,
			want: `{"content":"[LINK]("}`,
		},
		{
			in:   `{ "content": "[LINK](", "key": true }`,
			want: `{"content":"[LINK](","key":true}`,
		},
		{
			in: "```json\n" +
				"{\n" +
				"	\"key\": \"value\"\n" +
				"}\n" +
				"```",
			want: `{"key":"value"}`,
		},
		{
			in:   "````{ \"key\": \"value\" }```",
			want: `{"key": "value"}`,
		},
		{
			in:   `{"real_content": "Some string: Some other string Some string <a href=\"https://domain.com\">Some  link</a>"}`,
			want: `{"real_content":"Some string: Some other string Some string <a href=\"https://domain.com\">Some  link</a>"}`,
		},
		{
			in:   "{\"key\\_1\n\": \"value\"}",
			want: `{"key_1":"value"}`,
		},
		{
			in:   "{\"key\t\\_\": \"value\"}",
			want: `{"key\t_": "value"}`,
		},
		{
			in:   `{""answer"":[{""traits"":''Female aged 60+'',""answer1"":""5""}]}`,
			want: `{"answer":[{"traits":"Female aged 60+","answer1":"5"}]}`,
		},
		{
			in:   `{ "words": abcdef", "numbers": 12345", "words2": ghijkl" }`,
			want: `{"words":"abcdef","numbers":12345,"words2":"ghijkl"}`,
		},
		{
			in: `
				{
				  "resourceType": "Bundle",
				  "id": "1",
				  "type": "collection",
				  "entry": [
					{
					  "resource": {
						"resourceType": "Patient",
						"id": "1",
						"name": [
						  {"use": "official", "family": "Corwin", "given": ["Keisha", "Sunny"], "prefix": ["Mrs."},
						  {"use": "maiden", "family": "Goodwin", "given": ["Keisha", "Sunny"], "prefix": ["Mrs."]}
						]
					  }
					}
				  ]
				}
				`,
			want: `{"resourceType": "Bundle", "id": "1", "type": "collection", "entry": [{"resource": {"resourceType": "Patient", "id": "1", "name": [{"use": "official", "family": "Corwin", "given": ["Keisha", "Sunny"], "prefix": ["Mrs."]}, {"use": "maiden", "family": "Goodwin", "given": ["Keisha", "Sunny"], "prefix": ["Mrs."]}]}}]}`,
		},
		{
			in:   `{"html": "<h3 id="aaa">Waarom meer dan 200 Technical Experts - "Passie voor techniek"?</h3>"}`,
			want: `{"html":"<h3 id=\"aaa\">Waarom meer dan 200 Technical Experts - \"Passie voor techniek\"?</h3>"}`,
		},
		{
			in:   `{"key": .25}`,
			want: `{"key": 0.25}`,
		},

		{
			in:   `{  'reviews': [    {      'version': 'new',      'line': 1,      'severity': 'Minor',      'issue_type': 'Standard practice suggestion',      'issue': 'The merge request description is missing a link to the original issue or bug report.',      'suggestions': 'Add a link to the original issue or bug report in the *Issue* section.'    },    {      'version': 'new',      'line': 2,      'severity': 'Minor',      'issue_type': 'Standard practice suggestion',      'issue': 'The merge request description is missing a description of the critical issue or bug being addressed.',      'suggestions': 'Add a description of the critical issue or bug being addressed in the *Problem* section.'    } ]`,
			want: `{"reviews":[{"issue":"The merge request description is missing a link to the original issue or bug report.","issue_type":"Standard practice suggestion","line":1,"severity":"Minor","suggestions":"Add a link to the original issue or bug report in the *Issue* section.","version":"new"},{"issue":"The merge request description is missing a description of the critical issue or bug being addressed.","issue_type":"Standard practice suggestion","line":2,"severity":"Minor","suggestions":"Add a description of the critical issue or bug being addressed in the *Problem* section.","version":"new"}]}`,
		},
		{
			in:   `{"key":"",}`,
			want: `{"key":""}`,
		},
		{
			in:   "```json{\"array_key\": [{\"item_key\": 1\n}], \"outer_key\": 2}```",
			want: `{"array_key": [{"item_key": 1}], "outer_key": 2}`,
		},

		{
			in: `[
	{"Master""господин"}
	]`,
			want: `[{"Master":"господин"}]`,
		},
		// Issue #19: Stack overflow with trailing invalid characters
		{
			in: `
    {
      "Be": "",
      "gone": ""
    }
    ",п"г`,
			want: `{"Be":"","gone":""}`,
		},
		// Issue #18: Unescaped quotes inside string values
		{
			in:   `{"name": "John is "good",hah", "age": 30}`,
			want: `{"name":"John is \"good\",hah","age":30}`,
		},
		// PR #21: Extra '}' instead of ']' should not lose fields after array
		{
			in:   `{"items":[{"query":"smart phone","category":["smartphone"],"boost":{"tags":["flagship","5G","high-performance"],"ageGroup":"young_adult","gender":"male","brand":["Apple","Samsung","Google"],"price":{"min":800,"max":1500}},"filter":{"tags":["premium"],"gender":"male","brand":["Apple","Samsung","Google"],"price":{"min":800}}}}],"size":50}`,
			want: `{"items":[{"boost":{"ageGroup":"young_adult","brand":["Apple","Samsung","Google"],"gender":"male","price":{"max":1500,"min":800},"tags":["flagship","5G","high-performance"]},"category":["smartphone"],"filter":{"brand":["Apple","Samsung","Google"],"gender":"male","price":{"min":800},"tags":["premium"]},"query":"smart phone"}],"size":50}`,
		},
	}

	caseNo := 1
	for _, tt := range tests {
		t.Run("CASE-"+strconv.Itoa(caseNo), func(t *testing.T) {
			t.Log(tt.in)
			got1, err := RepairJSON(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if !jsonStringsEqual(got1, tt.want) {
				t.Errorf("RepairJSON() = %v, want %v, param in is %v", got1, tt.want, tt.in)
			}

			got2 := MustRepairJSON(tt.in)
			if !jsonStringsEqual(got2, tt.want) {
				t.Errorf("RepairJSON() = %v, want %v, param in is %v", got2, tt.want, tt.in)
			}
		})
		caseNo++
	}
}

// jsonStringsEqual
//
//	Description:
//	param jsonStr1
//	param jsonStr2
//	return bool
func jsonStringsEqual(jsonStr1, jsonStr2 string) bool {
	var jsonObj interface{}
	err := json.Unmarshal([]byte(jsonStr1), &jsonObj)
	if err != nil {
		return false
	}

	var jsonObj2 interface{}
	err = json.Unmarshal([]byte(jsonStr2), &jsonObj2)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(jsonObj, jsonObj2)
}
