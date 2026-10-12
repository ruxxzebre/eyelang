//go:build js && wasm

package main

import (
	"errors"
	"syscall/js"

	"eyelang/internal/playground"
)

var errNoOp = errors.New("eye.call needs an op, such as \"check\"")

// main puts eye.call on the page's globalThis, calls globalThis.eyeReady
// if the page set one, and keeps the program alive for the calls to come.
func main() {
	p := playground.New()
	fn := js.FuncOf(func(this js.Value, argv []js.Value) any {
		if len(argv) < 1 {
			return errorJSON(errNoOp)
		}
		raw := ""
		if len(argv) > 1 && argv[1].Type() == js.TypeString {
			raw = argv[1].String()
		}
		var out func(stream, text string)
		if len(argv) > 2 && argv[2].Type() == js.TypeFunction {
			cb := argv[2]
			out = func(stream, text string) { cb.Invoke(stream, text) }
		}
		return call(p, argv[0].String(), raw, out)
	})
	eye := js.Global().Get("Object").New()
	eye.Set("call", fn)
	js.Global().Set("eye", eye)
	if ready := js.Global().Get("eyeReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}
