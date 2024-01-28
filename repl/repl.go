package repl

import (
	"bufio"
	"eyelang/lexer"
	"eyelang/parser"
	"fmt"
	"io"
	"strings"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)

	for {
		_, err := fmt.Fprint(out, PROMPT)

		if err != nil {
			panic(err)
		}

		scanned := scanner.Scan()

		if !scanned {
			return
		}

		line := scanner.Text()
		l := lexer.New(line)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors(), line)
			continue
		}

		io.WriteString(out, program.String())
		io.WriteString(out, "\n")
	}
}

// TODO: polish when ILLEGAL (arrow up, etc)
func printParserErrors(out io.Writer, errors []parser.ParsingError, line string) {
	red := "\033[31m"
	reset := "\033[0m"

	prefix := "parsing error: "
	defaultPad := "\t"
	for _, error := range errors {
		leftPadLen := len(PROMPT) + error.Token.Position.Col - 1
		if leftPadLen >= len(prefix) {
			leftPad := strings.Repeat(" ", leftPadLen-len(prefix))
			io.WriteString(out, red+defaultPad+PROMPT+line+reset+"\n")
			io.WriteString(out, defaultPad+prefix+leftPad+"↑ "+error.Message+"\n")
		} else {
			leftPad := strings.Repeat(" ", len(PROMPT)+error.Token.Position.Col-1)
			io.WriteString(out, defaultPad+prefix+"\n")
			io.WriteString(out, defaultPad+PROMPT+line+"\n")
			io.WriteString(out, defaultPad+leftPad+"↑ "+error.Message+"\n")
		}
	}
}
