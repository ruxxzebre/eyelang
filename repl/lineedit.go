package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"
)

var ErrInterrupted = errors.New("interrupted")

type LineEditor struct {
	in          *os.File
	out         *os.File
	fd          int
	history     []string
	histMax     int
	histPath    string
	reader      *bufio.Reader
	cookedState *term.State
}

func NewLineEditor(in, out *os.File, histPath string) *LineEditor {
	fd := int(in.Fd())
	cooked, _ := term.GetState(fd)
	return &LineEditor{
		in:          in,
		out:         out,
		fd:          fd,
		histMax:     500,
		histPath:    histPath,
		reader:      bufio.NewReader(in),
		cookedState: cooked,
	}
}

func (e *LineEditor) RestoreTerminal() {
	if e.cookedState != nil {
		term.Restore(e.fd, e.cookedState)
	}
}

func (e *LineEditor) LoadHistory() error {
	if e.histPath == "" {
		return nil
	}
	data, err := os.ReadFile(e.histPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, line := range lines {
		if line != "" {
			e.history = append(e.history, line)
		}
	}
	return nil
}

func (e *LineEditor) SaveHistory() error {
	if e.histPath == "" {
		return nil
	}
	h := e.history
	if len(h) > e.histMax {
		h = h[len(h)-e.histMax:]
	}
	return os.WriteFile(e.histPath, []byte(strings.Join(h, "\n")+"\n"), 0o600)
}

func (e *LineEditor) AppendHistory(line string) {
	if line == "" {
		return
	}
	if n := len(e.history); n > 0 && e.history[n-1] == line {
		return
	}
	e.history = append(e.history, line)
}

func (e *LineEditor) Close() error { return nil }

type key int

const (
	keyRune key = iota
	keyEnter
	keyBackspace
	keyDelete
	keyLeft
	keyRight
	keyUp
	keyDown
	keyHome
	keyEnd
	keyCtrlC
	keyCtrlD
	keyCtrlL
	keyCtrlU
	keyCtrlW
	keyUnknown
)

func (e *LineEditor) readKey() (key, rune, error) {
	r, _, err := e.reader.ReadRune()
	if err != nil {
		return keyUnknown, 0, err
	}
	switch r {
	case '\r', '\n':
		return keyEnter, 0, nil
	case 0x7f, 0x08:
		return keyBackspace, 0, nil
	case 0x03:
		return keyCtrlC, 0, nil
	case 0x04:
		return keyCtrlD, 0, nil
	case 0x0c:
		return keyCtrlL, 0, nil
	case 0x15:
		return keyCtrlU, 0, nil
	case 0x17:
		return keyCtrlW, 0, nil
	case 0x01:
		return keyHome, 0, nil
	case 0x05:
		return keyEnd, 0, nil
	case 0x1b:
		return e.readEscape()
	}
	if r < 0x20 {
		return keyUnknown, 0, nil
	}
	return keyRune, r, nil
}

func (e *LineEditor) readEscape() (key, rune, error) {
	b, err := e.reader.ReadByte()
	if err != nil {
		return keyUnknown, 0, err
	}
	if b != '[' && b != 'O' {
		return keyUnknown, 0, nil
	}
	var seq []byte
	seq = append(seq, b)
	for {
		c, err := e.reader.ReadByte()
		if err != nil {
			return keyUnknown, 0, err
		}
		seq = append(seq, c)
		if c >= 0x40 && c <= 0x7e {
			break
		}
		if len(seq) > 8 {
			return keyUnknown, 0, nil
		}
	}
	switch string(seq) {
	case "[A":
		return keyUp, 0, nil
	case "[B":
		return keyDown, 0, nil
	case "[C":
		return keyRight, 0, nil
	case "[D":
		return keyLeft, 0, nil
	case "[H", "OH", "[1~", "[7~":
		return keyHome, 0, nil
	case "[F", "OF", "[4~", "[8~":
		return keyEnd, 0, nil
	case "[3~":
		return keyDelete, 0, nil
	}
	return keyUnknown, 0, nil
}

type lineState struct {
	buf     []rune
	cur     int
	histIdx int
	draft   []rune
	hasDraft bool
}

func (e *LineEditor) Prompt(prompt string) (string, error) {
	oldState, err := term.MakeRaw(e.fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(e.fd, oldState)

	st := &lineState{histIdx: len(e.history)}
	e.redraw(prompt, st)

	for {
		k, r, err := e.readKey()
		if err != nil {
			if err == io.EOF {
				return "", io.EOF
			}
			return "", err
		}
		switch k {
		case keyEnter:
			fmt.Fprint(e.out, "\r\n")
			return string(st.buf), nil
		case keyCtrlC:
			fmt.Fprint(e.out, "^C\r\n")
			return "", ErrInterrupted
		case keyCtrlD:
			if len(st.buf) == 0 {
				fmt.Fprint(e.out, "\r\n")
				return "", io.EOF
			}
		case keyCtrlL:
			fmt.Fprint(e.out, "\x1b[2J\x1b[H")
		case keyCtrlU:
			st.buf = st.buf[:0]
			st.cur = 0
		case keyCtrlW:
			deletePrevWord(st)
		case keyHome:
			st.cur = 0
		case keyEnd:
			st.cur = len(st.buf)
		case keyLeft:
			if st.cur > 0 {
				st.cur--
			}
		case keyRight:
			if st.cur < len(st.buf) {
				st.cur++
			}
		case keyUp:
			e.historyPrev(st)
		case keyDown:
			e.historyNext(st)
		case keyBackspace:
			if st.cur > 0 {
				st.buf = append(st.buf[:st.cur-1], st.buf[st.cur:]...)
				st.cur--
			}
		case keyDelete:
			if st.cur < len(st.buf) {
				st.buf = append(st.buf[:st.cur], st.buf[st.cur+1:]...)
			}
		case keyRune:
			st.buf = append(st.buf, 0)
			copy(st.buf[st.cur+1:], st.buf[st.cur:])
			st.buf[st.cur] = r
			st.cur++
		}
		e.redraw(prompt, st)
	}
}

func (e *LineEditor) redraw(prompt string, st *lineState) {
	fmt.Fprint(e.out, "\r\x1b[K"+prompt+string(st.buf))
	if back := len(st.buf) - st.cur; back > 0 {
		fmt.Fprintf(e.out, "\x1b[%dD", back)
	}
}

func (e *LineEditor) historyPrev(st *lineState) {
	if st.histIdx == 0 {
		return
	}
	if !st.hasDraft {
		st.draft = append([]rune(nil), st.buf...)
		st.hasDraft = true
	}
	st.histIdx--
	st.buf = []rune(e.history[st.histIdx])
	st.cur = len(st.buf)
}

func (e *LineEditor) historyNext(st *lineState) {
	if st.histIdx >= len(e.history) {
		return
	}
	st.histIdx++
	if st.histIdx == len(e.history) {
		if st.hasDraft {
			st.buf = append([]rune(nil), st.draft...)
		} else {
			st.buf = st.buf[:0]
		}
		st.cur = len(st.buf)
		return
	}
	st.buf = []rune(e.history[st.histIdx])
	st.cur = len(st.buf)
}

func deletePrevWord(st *lineState) {
	if st.cur == 0 {
		return
	}
	i := st.cur
	for i > 0 && unicode.IsSpace(st.buf[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(st.buf[i-1]) {
		i--
	}
	st.buf = append(st.buf[:i], st.buf[st.cur:]...)
	st.cur = i
}
