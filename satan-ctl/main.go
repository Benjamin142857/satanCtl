package main

import (
	"fmt"
	"os"
)

const version = "v1.0.0"

type StCommand interface {
	ParseArgs(args []string) error
	Description() string
	Exec()
}

var StCmdMap = map[string]StCommand{
	"st2go": St2Go,
}

func help() {
	fmt.Println("NAME:")
	fmt.Printf("\tSATAN command-line management tool.\n\n")

	fmt.Println("VERSION:")
	fmt.Printf("\t%v\n\n", version)

	fmt.Println("COMMANDS:")
	for k, v := range StCmdMap {
		fmt.Printf("\t%v%v\n\n", k, v.Description())
	}
}

func dispatch(cmd string, args []string) {
	if (cmd == "help") || (cmd == "-h") {
		help()
	} else if (cmd == "version") || (cmd == "-v") {
		fmt.Printf("SatanCtl version %v\n", version)
	} else if stCmd := StCmdMap[cmd]; stCmd != nil {
		if err := stCmd.ParseArgs(args); err != nil {
			return
		}
		stCmd.Exec()
	} else {
		fmt.Printf("unknown command \"%v\", try \"help\".\n", cmd)
	}
}

func main() {
	if len(os.Args) < 2 {
		help()
		return
	}

	dispatch(os.Args[1], os.Args[2:])
}
