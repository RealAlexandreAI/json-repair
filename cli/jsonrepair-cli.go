package main

import (
	"flag"
	"fmt"
	"github.com/RealAlexandreAI/json-repair"
	"os"
)

const AppVersion = "0.0.11"

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

	switch {
	case input != "":
		fmt.Println(jsonrepair.MustRepairJSON(input))
	case file != "":
		fi, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("[json-repair] invalid file path: %s", file)
			fmt.Println()
			return
		}
		fmt.Println(jsonrepair.MustRepairJSON(string(fi)))
	default:
		fmt.Println("{}")
	}

}
