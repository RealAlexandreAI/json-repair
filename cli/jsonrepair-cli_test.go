package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func Test_cliInner_v(t *testing.T) {

	os.Args = append(os.Args, "-v")

	rst := cliInner()

	if !strings.Contains(rst, "Version") {
		t.Errorf("-v ut error.")
	}

	os.Args = os.Args[:len(os.Args)-1]
	resetVars()

}

func Test_cliInner_h(t *testing.T) {

	os.Args = append(os.Args, "-h")

	rst := cliInner()

	if rst != "" {
		t.Errorf("-h ut error.")
	}

	os.Args = os.Args[:len(os.Args)-1]
	resetVars()

}

func Test_cliInner_i(t *testing.T) {

	os.Args = append(os.Args, "-i")
	os.Args = append(os.Args, "{'employees':['John', 'Anna', ")

	rst := cliInner()

	if !jsonStringsEqual(rst, `{"employees":["John","Anna"]}`) {
		t.Errorf("-i ut error.")
	}

	os.Args = os.Args[:len(os.Args)-2]
	resetVars()

}

func Test_cliInner_f(t *testing.T) {

	tmpFile := writeToTemp("{'employees':['John', 'Anna', ")

	os.Args = append(os.Args, "-f")
	os.Args = append(os.Args, tmpFile)

	rst := cliInner()

	if !jsonStringsEqual(rst, `{"employees":["John","Anna"]}`) {
		t.Errorf("-i ut error.")
	}

	os.Args = os.Args[:len(os.Args)-2]
	resetVars()
}

func Test_cliInner_i_f(t *testing.T) {

	tmpFile := writeToTemp("{'employees':['John', 'Anna', ")

	os.Args = append(os.Args, "-f")
	os.Args = append(os.Args, tmpFile)
	os.Args = append(os.Args, "-i")
	os.Args = append(os.Args, "")

	rst := cliInner()

	if !jsonStringsEqual(rst, `{"employees":["John","Anna"]}`) {
		t.Errorf("-i ut error.")
	}

	os.Args = os.Args[:len(os.Args)-4]
	resetVars()
}

func resetVars() {
	versionFlag = false
	helpFlag = false
	input = ""
	file = ""
}

func writeToTemp(input string) string {
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102150405")
	fileName := fmt.Sprintf("employees_%s.json", timestamp)
	filePath := filepath.Join(tempDir, fileName)
	data := []byte(input)
	err := ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		fmt.Println("Error writing to file:", err)
	}
	return filePath
}

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
