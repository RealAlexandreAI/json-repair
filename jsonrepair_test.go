package jsonrepair

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

// Test_Example
//
//	@Description:
//	@param t
func Test_Example(t *testing.T) {
	in := "```json {'employees':['John', 'Anna', ```"

	rst := RepairJSON(in)
	fmt.Println(rst)
}

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
			in:   "'\"'",
			want: `"\""`,
		},
		{
			in:   "'string\"",
			want: `"string\""`,
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
			want: `{"value_1":"value_2"}`,
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
			in:   `{""answer":[{""traits":""Female aged 60+",""answer1":""5"}]}`,
			want: `{"answer":[{"traits":"Female aged 60+","answer1":"5"}]}`,
		},
		{
			in:   `{"key":"",}`,
			want: `{"key":",}"}`,
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
			in:   `{\n"html": "<h3 id="aaa">Waarom meer dan 200 Technical Experts - "Passie voor techniek"?</h3>"}`,
			want: `{"html":"<h3 id=\"aaa\">Waarom meer dan 200 Technical Experts - \"Passie voor techniek\"?</h3>"}`,
		},
	}

	caseNo := 0
	for _, tt := range tests {
		t.Run("CASE-"+strconv.Itoa(caseNo), func(t *testing.T) {
			got := RepairJSON(tt.in)
			if !jsonStringsEqual(got, tt.want) {
				t.Errorf("RepairJSON() = %v, want %v, param in is %v", got, tt.want, tt.in)
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
