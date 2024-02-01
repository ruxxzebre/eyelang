package main

import (
	"eyelang/cli"
	"eyelang/object"
	"eyelang/repl"
	"flag"
	"fmt"
	"os"
	"os/user"
)

func main() {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	evalFileFlag := flag.String("evf", "", "evaluate a file")
	evalStringFlag := flag.String("evs", "", "evaluate a string")
	replFlag := flag.Bool("repl", false, "launch into repl")

	flag.Parse()

	baseEnv := object.NewEnvironment()

	if *evalStringFlag != "" {
		baseEnv.Load(cli.EvalString(*evalStringFlag))
	}

	if *evalFileFlag != "" {
		baseEnv.Load(cli.EvalFile(*evalFileFlag))
	}

	if *replFlag {
		fmt.Printf("eyelang.init\n")
		fmt.Printf("Hey %s! Welcome to Eye Language REPL\n", user.Username)
		repl.StartWithEnv(os.Stdin, os.Stdout, baseEnv)
		return
	}

	fmt.Printf("eyelang.init\n")
	fmt.Printf("Hey %s! Welcome to Eye Language REPL\n", user.Username)
	repl.Start(os.Stdin, os.Stdout)
}
