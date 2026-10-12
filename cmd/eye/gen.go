package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"eyelang/check"
	"eyelang/internal/netpolicy"
)

// genTarget is one kind of output `eye gen` writes from a program.
type genTarget struct {
	name    string
	summary string
	run     func(args []string, stdout, stderr io.Writer) int
	usage   func(w io.Writer) // what `eye gen <target> -h` prints, flags included
}

var genTargets []genTarget

func init() {
	genTargets = []genTarget{
		{name: "policy", summary: "Kubernetes network policy: which module may reach which service on which port", run: runGenPolicy, usage: genPolicyUsage},
	}
}

// runGen implements `eye gen <target> [flags] [program-directory]`.
func runGen(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		genUsage(stderr)
		return exitUsage
	}
	for _, t := range genTargets {
		if t.name == args[0] {
			return t.run(args[1:], stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "eye gen: unknown target %q\n\n", args[0])
	genUsage(stderr)
	return exitUsage
}

func genUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: eye gen <target> [flags] [program-directory]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Writes build output from a program's contracts to standard output. The program")
	fmt.Fprintln(w, "must check; with no directory, it is the program in the current directory.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "targets:")
	for _, t := range genTargets {
		fmt.Fprintf(w, "  %-8s %s\n", t.name, t.summary)
	}
	for _, t := range genTargets {
		fmt.Fprintln(w)
		t.usage(w)
	}
}

func genPolicyFlags(w io.Writer) (*flag.FlagSet, *string) {
	flags := flag.NewFlagSet("eye gen policy", flag.ContinueOnError)
	flags.SetOutput(w)
	namespace := flags.String("namespace", "", "the Kubernetes `namespace` the policies go in, such as homelab; without it they have none, and kubectl apply -n decides")
	return flags, namespace
}

func genPolicyUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: eye gen policy [--namespace name] [program-directory]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Writes a Kubernetes NetworkPolicy (networking.k8s.io/v1) for each contract with")
	fmt.Fprintln(w, "@port: the pods of the module that serves it accept TCP on that port only from")
	fmt.Fprintln(w, "the pods of the modules whose code calls its operations, among those its @allow")
	fmt.Fprintln(w, "lets call it. A module's pods carry the label eyelang.module: <module>, its")
	fmt.Fprintln(w, "import path; the program's own module goes by the name of the program's")
	fmt.Fprintln(w, "directory. When nothing in the program serves a contract, the policy selects")
	fmt.Fprintln(w, "the pods labelled with the contract's own module, such as eyelang.module: notify")
	fmt.Fprintln(w, "for a Notify written in another language. Tests, events and contracts without")
	fmt.Fprintln(w, "@port, which are services outside the network, get no rule; nor does egress.")
	fmt.Fprintln(w, "Each policy is named after the program and the contract, such as lab-notify,")
	fmt.Fprintln(w, "so the policies of programs that call one service add up. For example:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  eye gen policy --namespace homelab testdata/lab | kubectl apply -f -")
	fmt.Fprintln(w)
	flags, _ := genPolicyFlags(w)
	flags.PrintDefaults()
}

// runGenPolicy implements `eye gen policy [--namespace name] [program-directory]`.
func runGenPolicy(args []string, stdout, stderr io.Writer) int {
	flags, namespace := genPolicyFlags(stderr)
	flags.Usage = func() { genPolicyUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return exitUsage
	}
	dir := "."
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	}
	if info, err := os.Stat(dir); err != nil {
		fmt.Fprintf(stderr, "eye gen policy: %v\n", err)
		return exitErrors
	} else if !info.IsDir() {
		fmt.Fprintf(stderr, "eye gen policy: %s is a file; give the program's directory\n", dir)
		return exitUsage
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(stderr, d)
		}
		return exitErrors
	}
	if err := netpolicy.Write(stdout, prog, netpolicy.Options{Source: filepath.Clean(dir), Namespace: *namespace}); err != nil {
		fmt.Fprintf(stderr, "eye gen policy: %v\n", err)
		return exitUsage
	}
	return 0
}
