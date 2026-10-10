package repl

import (
	"bufio"
	"eyelang/eval"
	"eyelang/lexer"
	"eyelang/object"
	"eyelang/parser"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

const PROMPT = ">> "
const historyFileName = ".eyelang_history"

func StartWithEnv(in io.Reader, out io.Writer, env *object.Environment) {
	if f, ok := in.(*os.File); ok && isTerminal(f) {
		startInteractive(f, out, env)
		return
	}
	startScripted(in, out, env)
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func startInteractive(in *os.File, out io.Writer, env *object.Environment) {
	outFile, ok := out.(*os.File)
	if !ok {
		outFile = os.Stdout
	}

	editor := NewLineEditor(in, outFile, historyFilePath())
	editor.LoadHistory()
	defer editor.SaveHistory()
	defer editor.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		editor.RestoreTerminal()
		editor.SaveHistory()
		os.Exit(130)
	}()
	defer signal.Stop(sigCh)

	for {
		input, err := editor.Prompt(PROMPT)
		if err == ErrInterrupted {
			continue
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Fprintln(out, "input error:", err)
			return
		}

		if strings.TrimSpace(input) == "" {
			continue
		}

		editor.AppendHistory(input)
		evalLine(out, input, env)
	}
}

func startScripted(in io.Reader, out io.Writer, env *object.Environment) {
	scanner := bufio.NewScanner(in)
	for {
		if _, err := fmt.Fprint(out, PROMPT); err != nil {
			panic(err)
		}
		if !scanner.Scan() {
			return
		}
		evalLine(out, scanner.Text(), env)
	}
}

func evalLine(out io.Writer, input string, env *object.Environment) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		printParserErrors(out, p.Errors(), input)
		return
	}
	evaluated := eval.Eval(program, env)
	if evaluated != nil {
		fmt.Fprintln(out, evaluated.Inspect())
	}
}

func historyFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, historyFileName)
}

// TODO: polish when ILLEGAL (arrow up, etc)
func printParserErrors(out io.Writer, errors []parser.ParsingError, _line string) {
	prompt := []rune(PROMPT)

	for _, error := range errors {
		leftPadLen := len(prompt) + error.Token.Position.Col - 1
		leftPad := strings.Repeat(" ", leftPadLen)
		io.WriteString(out, leftPad+" | "+error.Message+"\n")
	}
}
