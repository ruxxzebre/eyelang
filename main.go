package main

import (
	"eyelang/cli"
	"eyelang/eval"
	"eyelang/repl"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"
)

func main() {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	evalFileFlag := flag.String("load", "", "evaluate a file")
	evalStringFlag := flag.String("eval", "", "evaluate a string")
	replFlag := flag.Bool("repl", false, "launch into repl, includes scopes from -load and -eval")

	flag.Parse()

	replScope := eval.InitReplScope()
	baseEnv := replScope.Outer

	if *evalStringFlag != "" {
		fmt.Println(*evalStringFlag)
		baseEnv.Load(cli.EvalString(*evalStringFlag))
	}

	if *evalFileFlag != "" {
		baseEnv.Load(cli.EvalFile(*evalFileFlag))
	}

	if !*replFlag && *evalStringFlag != "" || *evalFileFlag != "" {
		return // exit
	}

	lastArgIdx := len(os.Args) - 1
	if lastArgIdx > 0 && len(os.Args[lastArgIdx]) > 0 && !strings.HasPrefix(os.Args[lastArgIdx], "-") {
		cli.EvalFile(os.Args[lastArgIdx])
		if !*replFlag {
			return
		}
	}

	fmt.Printf("eyelang.init\n")
	fmt.Printf("Hey %s! Welcome to Eye Language REPL\n", user.Username)
	repl.StartWithEnv(os.Stdin, os.Stdout, baseEnv)
}
