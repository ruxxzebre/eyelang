package object

var BaseStore = map[string]Object{
	"builtins": &String{
		Value: `puts
len first last rest push
printFile
load import
builtins
`,
	},
}

type Environment struct {
	store map[string]Object
	outer *Environment
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	env := &Environment{s, nil}
	env.loadStore(BaseStore)

	return env
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}

	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func (env *Environment) Load(from *Environment) {
	panic("unused")
	for key, value := range from.store {
		env.store[key] = value
	}

	// from.outer = nil
	// from.store = nil
}

func (env *Environment) loadStore(store map[string]Object) {
	for key, value := range store {
		env.store[key] = value
	}
}
