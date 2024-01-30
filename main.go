package main

import (
	"eyelang/cli"
	"eyelang/repl"
	"fmt"
	"os"
	"os/user"
)

func main() {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	if len(os.Args) > 1 {
		args := os.Args[1:]
		filepath := args[0]
		cli.EvalFile(filepath)
		return
	}

	fmt.Printf("eyelang.init\n")
	fmt.Printf("Hey %s! Welcome to Eye Language REPL\n", user.Username)
	repl.Start(os.Stdin, os.Stdout)
}
