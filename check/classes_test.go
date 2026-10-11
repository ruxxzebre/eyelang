package check

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// errorClasses lists every kind of error SPECS.md promises the checker
// reports, by section, with the test that shows it at its exact place: a
// case of TestDiagnostics, TestTypeDiagnostics or TestMigrationDiagnostics,
// or a test function. Each message says what was expected, what was found
// and where (CLAUDE.md, "Writing").
//
// Not checked yet, and so not listed: a contract's port and its allowed
// callers (phase 6), the Approval that @destructive commands and ending an
// entity need, recordings that lack a field the code reads (@unwitnessed),
// values captured by structured concurrency's tasks (3.5) and dyn (v0.2).
var errorClasses = []struct{ section, class, test string }{
	{"§4, §23", "a syntax error", "syntax errors"},
	{"§6", "a name nothing declares", "undefined names"},
	{"§6", "a name declared twice", "duplicate declarations"},
	{"§6", "_ outside a piped call", "_ goes in a piped call"},
	{"§6", "in: or for: alone", "in: and for: alone need a value"},
	{"§6", "return inside a lambda", "return leaves a function, not a lambda"},
	{"§6", "break or continue outside a loop", "break outside a loop"},
	{"§13", "a module that doesn't exist", "missing module"},
	{"§13", "a name a module doesn't have", "TestImportsAcrossModules"},
	{"§9, §13", "a variant two imported modules declare", "TestAmbiguousVariants"},
	{"§13", "a name, variant or method another module doesn't make pub", "TestModuleBoundaries"},
	{"§5", "an opaque type's fields or variants used outside its module", "TestModuleBoundaries"},
	{"§5", "a value of the wrong type", "a value of the wrong type"},
	{"§5", "an argument of the wrong type", "an argument of the wrong type"},
	{"§6", "too many, missing or unknown arguments", "arguments: too many, missing, unknown"},
	{"§5", "a record literal without a type, a missing or unknown field", "a record literal takes the type expected"},
	{"§5", "a field or method a type doesn't have", "fields and methods"},
	{"§5", "a String where a Key or Zone is needed, an unknown zone", "strings become keys and zones where those are expected"},
	{"§4", "an unknown zone in a time", "a time's zone is checked"},
	{"§11", "an implicit conversion", "nothing converts implicitly"},
	{"§7", "a change to a let", "a let can't change"},
	{"§7", "a mut argument that isn't a var", "a mut argument is a var"},
	{"§7", "a mut parameter passed without mut, or mut where none is", "a mut parameter is passed with mut, and only it"},
	{"§7", "a variable passed as mut twice, or as mut and read", "no value is changed through two names"},
	{"§7", "a mut self method on what can't change", "a mut self method needs a var"},
	{"§7", "a closure capturing a var", "closures don't capture vars"},
	{"§7", "a closure capturing a mut parameter", "a closure can't capture what can change"},
	{"§8", "a type variable used as a type", "a type variable stands for any type"},
	{"§8", "an impl for a type of another module", "impl gives methods to a type this module declares"},
	{"§8", "a where clause naming what isn't a type variable or a trait", "where names a type variable and a trait"},
	{"§8", "a trait inside a type", "a trait isn't a type inside a type"},
	{"§9", "a match missing a variant", "a match covers every variant"},
	{"§9", "a match missing a length of list", "a match covers every length of a list"},
	{"§9", "a match missing a nested case", "a match is exhaustive with nested patterns"},
	{"§9", "a let whose pattern can fail", "a let without else matches every value"},
	{"§9", "a pattern naming a variant of another type", "a pattern names a variant of the matched type"},
	{"§10", "? that can't wrap the error", "? wraps an error only in a variant that holds it"},
	{"§10", "? on what isn't a Result or Option, or in the wrong function", "? needs a Result or an Option, and a function that returns one"},
	{"§11", "a trait method without self", "a trait's methods take self"},
	{"§11", "an impl missing a method, or with an extra one", "an impl gives every method its trait needs, and no other"},
	{"§11", "an impl of what isn't a trait", "only a trait can be implemented"},
	{"§8", "a value of a type without the impl used as a dyn Trait", "a value is a dyn Trait only if its type implements the trait"},
	{"§8", "dyn with a trait whose methods are generic", "dyn takes a trait whose methods aren't generic"},
	{"§8", "dyn with a type", "dyn takes a trait"},
	{"§5", "a field a built-in type doesn't have", "a built-in type has only the fields it declares"},
	{"§11", "an impl in neither the trait's nor the type's module", "an impl lives with its trait or its type"},
	{"§11", "the same impl twice", "a type implements a trait once"},
	{"§11", "a trait given the wrong number of type arguments", "a trait's type arguments"},
	{"§11", "an impl method whose types differ from the trait's", "an impl's method has its trait's types"},
	{"§11", "a call whose types lack the callee's traits", "a call meets its callee's bounds"},
	{"§11", "a type variable used with a trait it doesn't require", "a type variable has only the traits it requires"},
	{"§11", "an operator a type doesn't implement", "operators need types that have them"},
	{"§11", "showing what has no Show", "a value shown in a string has Show"},
	{"§5", "{} where neither a map, a set nor a record is expected", "{} is an empty map, set or record"},
	{"§5", "a field, method or index a type doesn't have, or a type a value can't have", "every type has only its own fields and methods"},
	{"§11", "a default method using what its trait doesn't have", "a trait's default method sees self as the trait"},
	{"§8", "a collection function given what isn't a collection it takes, or arguments it doesn't take", "a collection function takes the collections and arguments it declares"},
	{"§13", "a built-in given a function of the wrong type, or an argument it doesn't take", "a built-in that takes a function is checked against its declaration"},
	{"§12", "a function of the wrong function type", "a function type is checked like any type"},
	{"§11", "a function shown, compared or saved", "a function has none of the derived traits"},
	{"§12", "a public function's unstated effects", "a public function states its effects"},
	{"§12", "effects through private functions", "effects pass through private functions"},
	{"§12", "effects through a function named as a value, and in a job, its calls that aren't steps", "effects pass through functions named as values"},
	{"§12", "a waiting call where nothing waits", "waiting calls only in jobs"},
	{"§12", "a durable step without !", "durable steps in jobs need !"},
	{"§12", "a waiting function called without !", "a waiting function is called with !"},
	{"§12", "an entity call in a job without !", "entity calls in jobs are durable steps"},
	{"§12", "an outside call in a job without !, or ! on a pure call", "outside calls in jobs are durable steps"},
	{"§13", "a durable step in a task in a job", "a task in a job only computes"},
	{"§20", "with outside a test", "with only in tests"},
	{"§13", "a job started from main without a key", "main starts a job once, by key"},
	{"§14", "a change to an entity's identity", "state can change in entity methods, the identity can't"},
	{"§14", "a new version without its migration", "a new version needs a migration from the old one"},
	{"§14", "a gap in the migrations", "no gaps"},
	{"§14", "a migration to a version that doesn't exist", "migrations go to versions that exist"},
	{"§14", "a last migration that doesn't give the declared state", "the last migration gives the declared state"},
	{"§14", "an older migration of the wrong shape", "older migrations give the next version's shape, read by name"},
	{"§14", "a migration reading a field the one before doesn't give", "a migration reads what the one before gives"},
	{"§14", "a migration with effects", "a migration is pure"},
	{"§15", "a job argument that can't be saved", "a job's arguments are saved"},
	{"§15", "what can't be saved held across a !", "what can't be saved isn't held across a !"},
	{"§15", "a declared type holding what can't be saved", "what a job saves has Save, field by field"},
	{"§15", "an unknown or misplaced attribute", "attributes are declared"},
	{"§15", "an attribute's wrong argument", "attribute arguments"},
	{"§15", "a deadline at a time of day without its zone", "a deadline at a time of day needs its zone"},
	{"§15", "a schedule given the wrong type", "a schedule's time"},
	{"§16", "a view that doesn't start from entities", "views start from an entity set"},
	{"§17", "a command's Uncertain left unhandled", "a command may come back Uncertain"},
	{"§17", "a command's result used before it is reconciled", "an Uncertain result can't pass for an outcome"},
	{"§17", "reconcile without a @reconcile query", "reconcile needs a @reconcile query"},
	{"§17", "an event's value of the wrong type", "events carry their type"},
	{"§17", "a served contract missing an operation or changing its parameters", "a served contract serves every operation, as declared"},
	{"§17", "a trigger that isn't an event", "a trigger names a contract's event"},
	{"§19", "a model asked for a type without a Schema", "a model is asked for a type with a Schema"},
}

// TestEveryErrorClassIsTested checks that each error class names a test
// that exists.
func TestEveryErrorClassIsTested(t *testing.T) {
	files, _ := filepath.Glob("*_test.go")
	known := map[string]bool{}
	caseName := regexp.MustCompile(`(?m)^\t\t\{"([^"]+)"`)
	funcName := regexp.MustCompile(`(?m)^func (Test\w+)`)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range caseName.FindAllStringSubmatch(string(src), -1) {
			known[m[1]] = true
		}
		for _, m := range funcName.FindAllStringSubmatch(string(src), -1) {
			known[m[1]] = true
		}
	}
	for _, c := range errorClasses {
		if !known[c.test] {
			t.Errorf("%s %s: no test named %q", c.section, c.class, c.test)
		}
	}
}
