package parser

import (
	"fmt"
	"strings"
)

var traceLevel = 0
var silentTrace = true

const traceIdentPlaceholder = "\t"

func muteTracer() {
	silentTrace = true
}

func unmuteTracer() {
	silentTrace = false
}

func identLevel() string {
	return strings.Repeat(traceIdentPlaceholder, traceLevel-1)
}

func tracePrint(fs string, misc ...string) {
	if silentTrace {
		return
	}
	if len(misc) != 0 {
		fmt.Printf("%s%s (val: %s)\n", identLevel(), fs, strings.Join(misc, ","))
	} else {
		fmt.Printf("%s%s\n", identLevel(), fs)
	}
}

func trace(msg string, misc ...string) string {
	traceLevel++
	tracePrint("BEGIN "+msg, misc...)
	return msg
}

func untrace(msg string, misc ...string) {
	tracePrint("END "+msg, misc...)
	traceLevel--
}
