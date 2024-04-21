package main

import (
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/RealAlexandreAI/json-repair"
)

const AppVersion = "0.0.4"

var (
	versionFlag bool
	helpFlag    bool
	file        string
	input       string
)

// init
//
//	@Description:
func init() {
	flag.BoolVar(&versionFlag, "v", false, "Print version details")
	flag.BoolVar(&helpFlag, "h", false, "Print help")
	flag.StringVar(&input, "i", "{}", "String input inline")
	flag.StringVar(&file, "f", "", "File path")
}

// printDefaults
//
//	@Description:
func printDefaults() {
	fmt.Println("Usage: jsonrepair <options>")
	fmt.Println("Options:")
	flag.VisitAll(func(flag *flag.Flag) {
		fmt.Println("\t-"+flag.Name, "\t", flag.Usage, "(Default "+flag.DefValue+")")
	})
}

// main
//
//	@Description:
func main() {
	flag.Parse()

	if versionFlag {
		fmt.Println("Version:", AppVersion)
		return
	} else if helpFlag {
		printDefaults()
		return
	}

	if input != "" {
		fmt.Println(jsonrepair.RepairJSON(input))
	}

	if file != "" {

		if input != "" {
			fmt.Println("---")
		}

		fi, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Printf("[json-repair] invalid file path: %s", file)
			fmt.Println()
			return
		}
		fmt.Println(jsonrepair.RepairJSON(string(fi)))
	}
}
