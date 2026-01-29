package main

import (
	"flag"
	"fmt"
	"github.com/RealAlexandreAI/json-repair"
	"os"
)

// 通过 ldflags 在构建时注入版本号
// go build -ldflags "-X main.version=x.x.x"
var version string

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
	flag.StringVar(&input, "i", "", "String input inline")
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
	fmt.Print(cliInner())
}

// cliInner
//
//	Description:
//	return string
func cliInner() string {
	flag.Parse()

	if versionFlag {
		if version == "" {
			version = "dev"
		}
		return fmt.Sprintf("Version: %s", version)
	} else if helpFlag {
		printDefaults()
		return ""
	}

	switch {
	case input != "":
		return jsonrepair.MustRepairJSON(input)
	case file != "":
		fi, err := os.ReadFile(file)
		if err != nil {
			return fmt.Sprintf("[json-repair] invalid file path: %s", file)
		}
		return jsonrepair.MustRepairJSON(string(fi))
	default:
		return ""
	}
}
