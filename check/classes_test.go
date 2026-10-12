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
// Not checked yet, and so not listed: recordings that lack a field the code
// reads (@unwitnessed), values captured by structured concurrency's tasks
// (3.5) and dyn (v0.2).
var errorClasses = []struct{ section, class, test string }{
	{"§4, §23", "a syntax error", "syntax errors"},
	{"§5", "an Iter used as a list, or a function an Iter doesn't take", "an Iter's pipeline gives an Iter, and toList collects it"},
	{"§5", "a function with effects in an Iter's pipeline", "an Iter's functions only compute"},
	{"§5, §15", "an Iter compared, or live across a `!`, or walked by a loop that waits", "an Iter can't be compared, printed or kept across a wait"},
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
	{"§5", "a record literal in a lambda whose call's result nothing gives a type", "a lambda's record literal with nothing to say its type"},
	{"§5", "a field or method a type doesn't have", "fields and methods"},
	{"§5", "a String where a Key or Zone is needed, an unknown zone", "strings become keys and zones where those are expected"},
	{"§5", "an integer literal too large or too small for the fixed-width type it becomes", "a literal too large for its type"},
	{"§5", "a character outside ASCII where a Byte is expected", "a character outside ASCII where a Byte is expected"},
	{"§5", "+, -, *, / or % on a fixed-width integer", "arithmetic operators on fixed-width integers"},
	{"§5", "a fixed-width method on another type, or given another width", "fixed-width methods on other types"},
	{"§5", "a function a built-in type doesn't have, as Int32.parse", "a built-in type's function that doesn't exist"},
	{"§4", "an unknown zone in a time", "a time's zone is checked"},
	{"§11", "an implicit conversion", "nothing converts implicitly"},
	{"§10", "dividing an Int by what may be 0", "/ and % take a NonZero divisor"},
	{"§7", "a change to a let", "a let can't change"},
	{"§7", "a mut argument that isn't a var", "a mut argument is a var"},
	{"§7", "a mut parameter passed without mut, or mut where none is", "a mut parameter is passed with mut, and only it"},
	{"§7", "a variable passed as mut twice, or as mut and read", "no value is changed through two names"},
	{"§7", "a mut self method on what can't change", "a mut self method needs a var"},
	{"§7", "a closure that isn't a callback capturing a var", "a closure kept or returned doesn't capture a var"},
	{"§7", "a closure that isn't a callback capturing a mut parameter, or a mut self method's receiver", "a closure can't capture what can change"},
	{"§7", "a closure assigning a var it captures", "a callback can't assign a var"},
	{"§7", "a var a callback reads passed as mut to the same call", "a var a callback reads can't also be passed as mut"},
	{"§8", "a type variable used as a type", "a type variable stands for any type"},
	{"§5", "a variant where a type goes", "a variant isn't a type"},
	{"§6", "a call of a generic private function with types its body's operators or traits don't take", "a private function without types is generic, and each call meets its body's needs"},
	{"§8", "a type variable called or indexed, or an untyped parameter used as what a call can't be", "a type variable or an untyped parameter has only what its uses need"},
	{"§8", "a type variable a declaration doesn't declare", "a type variable is declared where it is used"},
	{"§8", "an impl for a type of another module", "impl gives methods to a type this module declares"},
	{"§8", "a where clause naming what isn't a type variable or a trait", "where names a type variable and a trait"},
	{"§8", "a trait inside a type", "a trait isn't a type inside a type"},
	{"§9", "a match missing a variant", "a match covers every variant"},
	{"§9", "a match missing a length of list", "a match covers every length of a list"},
	{"§9", "a match missing a nested case", "a match is exhaustive with nested patterns"},
	{"§9", "a let whose pattern can fail", "a let without else matches every value"},
	{"§9", "a pattern naming a variant of another type", "a pattern names a variant of the matched type"},
	{"§11", "a choice read with T.parse(s) matched with a variant missing, or used as what it isn't", "an enum's parse gives an Option of its variants, matched exhaustively"},
	{"§11", "a misspelt variant matched on T.parse(s)", "a misspelt variant in a match on an enum's parse"},
	{"§11", "T.variants() or T.parse(s) on a type with fields", "variants() and parse() only on a type whose variants have no fields"},
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
	{"§11", "a minus sign, or a built-in type's operator, that the type doesn't have", "a minus sign and the built-in operators need types that have them"},
	{"§6", "return with a value of the wrong type, or with what isn't a handler", "return gives the function's type, and with a handler"},
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
	{"§12", "a call of a function value whose type states effects the caller doesn't", "calling a function value has the effects its type states"},
	{"§12", "a function kept in data that does more than its type states, or a callback kept in data", "a function kept in data does only what its type states"},
	{"§8", "a method named like a field that holds a function", "a field that holds a function is called like a method"},
	{"§8", "x.f() where f doesn't take x first, takes no parameters, changes its first parameter, is only in an imported module, or x's type isn't known", "TestFunctionsAsMethods"},
	{"§12", "an impl's method doing more than its trait's states, or a private function more than its uses clause", "effects don't escape through traits or private uses clauses"},
	{"§12", "an effect that doesn't exist, or names what doesn't", "a uses clause lists effects that exist"},
	{"§12", "publishing, starting a child run or calling foreign code in a job without !, a step whose result can't be saved, or an unlisted publish, secret, consume or ffi effect", "publishing, starting a run, secrets and foreign code are steps and effects"},
	{"§12", "effects through a function named as a value, and in a job, its calls that aren't steps", "effects pass through functions named as values"},
	{"§12", "a waiting call where nothing waits", "waiting calls only in jobs"},
	{"§12", "a durable step without !", "durable steps in jobs need !"},
	{"§12", "a waiting function called without !", "a waiting function is called with !"},
	{"§12", "an entity call in a job without !", "entity calls in jobs are durable steps"},
	{"§12", "an outside call in a job without !, or ! on a pure call", "outside calls in jobs are durable steps"},
	{"§12", "an outside call in a waiting function without !, or ! on a pure call", "outside calls in waiting functions are durable steps"},
	{"§12", "a call to another service in a waiting function without !", "TestContractCallInWaitingFunction"},
	{"§13", "a durable step in a task in a job", "a task in a job only computes"},
	{"§13", "a durable step in a task in a waiting function", "a task in a waiting function only computes"},
	{"§15", "allDone! or firstDone! given what isn't a list of runs, all of runs, or all! or first!", "allDone! and firstDone! wait for a list of runs"},
	{"§15", "join on a run without !, or a run's outcome() read in a job", "a job joins a run, and a test or main reads its outcome"},
	{"§20", "with outside a test", "with only in tests"},
	{"§13", "a job started from main without a key", "main starts a job once, by key"},
	{"§14", "a change to an entity's identity", "state can change in entity methods, the identity can't"},
	{"§14", "ending an entity without an Approval, or from outside its own methods", "ending an entity needs an Approval"},
	{"§14", "a new version without its migration", "a new version needs a migration from the old one"},
	{"§14", "a gap in the migrations", "no gaps"},
	{"§14", "a migration to a version that doesn't exist", "migrations go to versions that exist"},
	{"§14", "a last migration that doesn't give the declared state", "the last migration gives the declared state"},
	{"§14", "an older migration of the wrong shape", "older migrations give the next version's shape, read by name"},
	{"§14", "a migration reading a field the one before doesn't give", "a migration reads what the one before gives"},
	{"§14", "a migration with effects", "a migration is pure"},
	{"§15", "a job argument that can't be saved", "a job's arguments are saved"},
	{"§15", "what can't be saved held across a !", "what can't be saved isn't held across a !"},
	{"§15", "what can't be saved, bound before a loop that waits and read in it", "what can't be saved isn't held across a ! on the next turn of a loop"},
	{"§12, §15", "what can't be saved held across a ! in a waiting function, its parameters included", "a waiting function holds nothing that can't be saved across a !"},
	{"§15", "a function that can't be saved given to a waiting function that keeps it across a !", "a waiting function holds nothing that can't be saved across a !"},
	{"§18", "a value of an extern type saved, shown or compared", "foreign code owns an extern type's values"},
	{"§18", "an extern fn without @library or @host, or with both", "an extern fn names where its code is, and takes what can cross"},
	{"§18", "a C function's parameter or result that can't cross to C", "an extern fn names where its code is, and takes what can cross"},
	{"§15", "a declared type holding what can't be saved", "what a job saves has Save, field by field"},
	{"§15", "an unknown or misplaced attribute", "attributes are declared"},
	{"§15", "an attribute's wrong argument", "attribute arguments"},
	{"§17", "@reconcile or an event's @key naming what doesn't exist", "attributes name what exists"},
	{"§15", "a deadline at a time of day without its zone", "a deadline at a time of day needs its zone"},
	{"§15", "a schedule given the wrong type", "a schedule's time"},
	{"§16", "a view that doesn't start from entities", "views start from an entity set"},
	{"§17", "a command's Uncertain left unhandled", "a command may come back Uncertain"},
	{"§17", "a command's result used before it is reconciled", "an Uncertain result can't pass for an outcome"},
	{"§17", "reconcile without a @reconcile query", "reconcile needs a @reconcile query"},
	{"§17", "a @fresh command called without basedOn", "a @fresh command needs basedOn"},
	{"§17", "a @fresh command's Stale or Expired left unhandled", "a match on a @fresh command's result handles Stale and Expired"},
	{"§17", "a @fresh command's result used as its declared type", "a @fresh command's result can't pass for its declared outcomes"},
	{"§17", "Stale answered by what isn't a @fresh command", "only a @fresh command's impl answers Stale"},
	{"§17", "@fresh on a result that isn't a variant type or has Stale or Expired, or a declared basedOn or approval", "@fresh needs a variant result without Stale or Expired, and adds names a command can't declare"},
	{"§17", "a @destructive command called without an Approval", "a @destructive command needs an Approval, which only approval! makes"},
	{"§17", "an event's value of the wrong type", "events carry their type"},
	{"§17", "a served contract missing an operation or changing its parameters", "a served contract serves every operation, as declared"},
	{"§17", "a trigger that isn't an event", "a trigger names a contract's event"},
	{"§17", "a fake's calls given what isn't an operation of a contract", "TestFakeCalls"},
	{"§17", "an operation called or used by a module its contract's @allow doesn't list", "a caller not in @allow"},
	{"§17", "an operation called by any module of the program under @allow(nobody)", "@allow(nobody) lets only the server call it"},
	{"§17", "a name in @allow that isn't a module, or nobody beside other names", "@allow names modules"},
	{"§17", "a served contract, or one with @allow, without @port, or a port out of range", "a served or guarded contract has a port"},
	{"§17", "the lab's call of Notify from a module taken out of its @allow", "TestLabAllowNeedsApproving"},
	{"§19", "a model asked for a type without a Schema", "a model is asked for a type with a Schema"},
	{"§19", "a tool that isn't a function declared with fn, or whose parameters or result have no Schema, or that waits", "a tool is a function declared with fn, with written types that have Schema"},
	{"§12, §19", "a public function that passes a tool without listing the tool's effects", "a public function that passes label lists net GitHub.label"},
	{"§19", "a call whose tools: names a @destructive tool without approve:", "a call that names a @destructive tool says who approves its calls"},
	{"§17, §19", "a @destructive function called without an Approval, or declaring approval itself", "@destructive adds approval: Approval, which code passes from approval!"},
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
