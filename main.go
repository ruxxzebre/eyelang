package main

import (
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

	fmt.Printf("Hey %s! Welcome to Eye Language REPL\n", user.Username)
	repl.Start(os.Stdin, os.Stdout)
}
