package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"eyelang/check"
	"eyelang/internal/codegen"
	"eyelang/internal/netpolicy"
	"eyelang/ir"
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
		{name: "go", summary: "Go types, client and server stub for each contract", run: func(args []string, stdout, stderr io.Writer) int {
			return runGenCode(goLang, args, stdout, stderr)
		}, usage: func(w io.Writer) { genCodeUsage(goLang, w) }},
		{name: "ts", summary: "TypeScript types, client and server stub for each contract", run: func(args []string, stdout, stderr io.Writer) int {
			return runGenCode(tsLang, args, stdout, stderr)
		}, usage: func(w io.Writer) { genCodeUsage(tsLang, w) }},
	}
}

// genLang is a language eye gen writes code in.
type genLang struct {
	target string // the target's name: go, ts
	name   string // Go, TypeScript
	write  func(*ir.Program, *ir.Module, codegen.Options) ([]byte, error)
	path   func(m *ir.Module) string // where --out puts a module's file
}

var (
	goLang = genLang{target: "go", name: "Go", write: codegen.Go, path: func(m *ir.Module) string {
		return filepath.Join(m.Name, m.Name+".go")
	}}
	tsLang = genLang{target: "ts", name: "TypeScript", write: codegen.TypeScript, path: func(m *ir.Module) string {
		return m.Name + ".ts"
	}}
)

type genCodeOpts struct {
	module, out, pkg string
}

func genCodeFlags(lang genLang, w io.Writer) (*flag.FlagSet, *genCodeOpts) {
	flags := flag.NewFlagSet("eye gen "+lang.target, flag.ContinueOnError)
	flags.SetOutput(w)
	o := &genCodeOpts{}
	flags.StringVar(&o.module, "module", "", "the `module` whose contracts to write, by import path or name, such as notify; without it, the program's only module with contracts")
	flags.StringVar(&o.out, "out", "", "a `directory` to write a file for each module with contracts into, instead of printing one")
	if lang.target == "go" {
		flags.StringVar(&o.pkg, "package", "", "the Go package's `name`; the module's name without it")
	}
	return flags, o
}

func genCodeUsage(lang genLang, w io.Writer) {
	fmt.Fprintf(w, "usage: eye gen %s [--module name] [--out directory]", lang.target)
	if lang.target == "go" {
		fmt.Fprint(w, " [--package name]")
	}
	fmt.Fprintln(w, " [program-directory]")
	fmt.Fprintln(w)
	switch lang.target {
	case "go":
		fmt.Fprintln(w, "Writes a Go package for a module's contracts: their types, which marshal to")
		fmt.Fprintln(w, "Eyelang's one JSON shape; for each contract a client, NotifyClient for")
		fmt.Fprintln(w, "Notify, that calls its operations over @http; and a server stub, the")
		fmt.Fprintln(w, "NotifyService interface to implement and NewNotifyHandler, an http.Handler")
		fmt.Fprintln(w, "that routes requests to it the way eye run serves the contract. A command's")
		fmt.Fprintln(w, "lost reply is an *Uncertain error, and a @fresh command's outcomes are")
		fmt.Fprintln(w, "ErrStale and ErrExpired.")
	default:
		fmt.Fprintln(w, "Writes a TypeScript module for a module's contracts: their types as the JSON")
		fmt.Fprintln(w, "they are on the wire, a variant type as a union discriminated on \"kind\"; for")
		fmt.Fprintln(w, "each contract a client class, NotifyClient for Notify, that calls its")
		fmt.Fprintln(w, "operations over @http; and a server stub, the NotifyService interface to")
		fmt.Fprintln(w, "implement and notifyHandler, a Fetch API handler from Request to Response")
		fmt.Fprintln(w, "that routes requests to it the way eye run serves the contract. It compiles")
		fmt.Fprintln(w, "with tsc --strict for ES2022 with the DOM library's fetch types.")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Operations without @http are left out of the client and the stub. The output")
	fmt.Fprintln(w, "is a build artifact, never committed. Without --out, one module's file goes")
	fmt.Fprintln(w, "to standard output; with it, each module with contracts gets a file, such as")
	fmt.Fprintf(w, "%s. For example:\n", lang.path(&ir.Module{Name: "notify"}))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  eye gen %s --module notify testdata/lab > %s\n", lang.target, filepath.Base(lang.path(&ir.Module{Name: "notify"})))
	fmt.Fprintln(w)
	flags, _ := genCodeFlags(lang, w)
	flags.PrintDefaults()
}

// runGenCode implements `eye gen go|ts [flags] [program-directory]`.
func runGenCode(lang genLang, args []string, stdout, stderr io.Writer) int {
	flags, o := genCodeFlags(lang, stderr)
	flags.Usage = func() { genCodeUsage(lang, stderr) }
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
	prefix := "eye gen " + lang.target
	if info, err := os.Stat(dir); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
		return exitErrors
	} else if !info.IsDir() {
		fmt.Fprintf(stderr, "%s: %s is a file; give the program's directory\n", prefix, dir)
		return exitUsage
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(stderr, d)
		}
		return exitErrors
	}
	mods := codegen.Modules(prog)
	if o.module != "" {
		m, err := codegen.FindModule(prog, o.module)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
			return exitUsage
		}
		mods = []*ir.Module{m}
	}
	switch {
	case len(mods) == 0:
		fmt.Fprintf(stderr, "%s: %s declares no contracts\n", prefix, dir)
		return exitErrors
	case o.out == "" && len(mods) > 1:
		var names []string
		for _, m := range mods {
			names = append(names, m.Path)
		}
		fmt.Fprintf(stderr, "%s: %d modules declare contracts (%s): name one with --module, or write them all with --out\n", prefix, len(mods), strings.Join(names, ", "))
		return exitUsage
	case o.pkg != "" && len(mods) > 1:
		fmt.Fprintf(stderr, "%s: --package names one package; give --module too\n", prefix)
		return exitUsage
	}
	opts := codegen.Options{Source: filepath.Clean(dir), Package: o.pkg}
	seen := map[string]string{}
	for _, m := range mods {
		src, err := lang.write(prog, m, opts)
		if err != nil {
			fmt.Fprintf(stderr, "%s: module %s: %v\n", prefix, m.Path, err)
			return exitErrors
		}
		if o.out == "" {
			_, _ = stdout.Write(src)
			continue
		}
		path := filepath.Join(o.out, lang.path(m))
		if other, ok := seen[path]; ok {
			fmt.Fprintf(stderr, "%s: modules %s and %s would both be written to %s; write each with --module\n", prefix, other, m.Path, path)
			return exitUsage
		}
		seen[path] = m.Path
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
			return exitErrors
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
			return exitErrors
		}
	}
	return 0
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
