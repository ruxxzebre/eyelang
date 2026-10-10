package main

import (
	"eyelang/cli"
	"eyelang/eval"
	"eyelang/repl"
	"flag"
	"fmt"
	"os"
	"os/user"
)

func main() {
	evalFileFlag := flag.String("load", "", "evaluate a file")
	evalStringFlag := flag.String("eval", "", "evaluate a string")
	replFlag := flag.Bool("repl", false, "launch into repl, includes scopes from -load and -eval")

	flag.Parse()

	ranSomething := *evalStringFlag != "" || *evalFileFlag != "" || len(flag.Args()) > 0
	startRepl := *replFlag || !ranSomething

	// scripts share one environment, which becomes the REPL's when it's requested
	scope := eval.InitScriptScope()
	if startRepl {
		scope = eval.InitReplScope()
	}
	env := scope.Outer

	failed := false
	report := func(err error) {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		}
	}

	if *evalStringFlag != "" {
		report(cli.EvalString(*evalStringFlag, env))
	}

	if *evalFileFlag != "" {
		report(cli.EvalFile(*evalFileFlag, env))
	}

	for _, arg := range flag.Args() {
		report(cli.EvalFile(arg, env))
		if !*replFlag {
			break // without -repl only the first script is run
		}
	}

	if !startRepl {
		if failed {
			os.Exit(1)
		}
		return
	}

	username := "there"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	fmt.Printf("eyelang.init\n")
	fmt.Printf("Hey %s! Welcome to Eye Language REPL\n", username)
	repl.StartWithEnv(os.Stdin, os.Stdout, env)
}
