package repl

import (
	"bufio"
	"eyelang/eval"
	"eyelang/lexer"
	"eyelang/object"
	"eyelang/parser"
	"fmt"
	"io"
	"strings"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()

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
		reader := strings.NewReader(line)
		runeReader := bufio.NewReader(reader)

		l := lexer.New_WIP(runeReader)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors(), line)
			continue
		}

		// FOR DEBUG:
		// io.WriteString(out, program.String())
		// io.WriteString(out, fmt.Sprintf("%#v\n", program))
		evaluated := eval.Eval(program, env)
		if evaluated != nil {
			fmt.Print(evaluated.Inspect())
			io.WriteString(out, "\n")
		}
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
