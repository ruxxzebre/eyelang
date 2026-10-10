// Package lsp is Eyelang's language server: `eye lsp` speaks the Language
// Server Protocol over standard input and output. It has no dependencies
// beyond the parser, so it starts instantly and answers in a millisecond or
// two.
//
// It offers diagnostics as you type, formatting, an outline of declarations,
// folding, and semantic highlighting. Hover, completion and go-to-definition
// need name resolution and come with phase 2.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

// message is any JSON-RPC 2.0 message: a request (ID and Method), a
// notification (Method only) or a response (ID with Result or Error).
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC error codes.
const (
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeRequestFailed  = -32803
)

// conn reads and writes messages framed with Content-Length headers.
type conn struct {
	r  *bufio.Reader
	mu sync.Mutex
	w  io.Writer
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{r: bufio.NewReader(r), w: w}
}

func (c *conn) read() (*message, error) {
	header, err := textproto.NewReader(c.r).ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	length, err := strconv.Atoi(strings.TrimSpace(header.Get("Content-Length")))
	if err != nil || length < 0 {
		return nil, fmt.Errorf("bad Content-Length header %q", header.Get("Content-Length"))
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, err
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("decoding a message: %w", err)
	}
	return &m, nil
}

func (c *conn) write(m *message) error {
	m.JSONRPC = "2.0"
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}

func (c *conn) notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(&message{Method: method, Params: raw})
}
