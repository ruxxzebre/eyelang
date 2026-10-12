package interp

// vec is a persistent vector: a sequence of values that is never changed in
// place. Setting the value at an index, or appending one, returns a new vec
// that shares almost everything with the old one, and the old one reads as
// it did. A map keeps its values in one (Map.vals), so that replacing a
// key's value takes time that grows with the logarithm of the map's size,
// not with its size.
//
// It is the vector of Clojure and Scala: a tree with 32 children to a node,
// whose leaves hold the values in order, and a tail of the last 1 to 32
// values outside the tree. Reading or setting an index walks from the root
// to one leaf, copying that path when setting: 3 nodes for 32,768 values, 4
// for a million. Appending writes into the tail, and puts the tail into the
// tree once it holds 32.
type vec struct {
	n     int
	shift uint    // vecBits times the levels between root and the leaves
	root  *vnode  // the values before the tail, in full leaves; nil if none
	tail  []Value // the last values, 1 to vecWidth of them; empty if n is 0
	// grow is shared by the vecs that share tail's array: how much of it
	// they use. Appending to the vec that uses all of it writes into the
	// spare room, as List.appended does.
	grow *growth
}

const (
	vecBits  = 5
	vecWidth = 1 << vecBits
	vecMask  = vecWidth - 1
)

// vnode is a node of a vec's tree. An inner node's children are *vnode, a
// leaf's are the values.
type vnode struct{ kids [vecWidth]Value }

// tailStart is the index of the first value in the tail.
func (v *vec) tailStart() int { return v.n - len(v.tail) }

// leaf returns the leaf that holds index i, which is before the tail.
func (v *vec) leaf(i int) *vnode {
	nd := v.root
	for s := v.shift; s > 0; s -= vecBits {
		nd = nd.kids[(i>>s)&vecMask].(*vnode)
	}
	return nd
}

// get returns the value at index i, which must be below n.
func (v *vec) get(i int) Value {
	if t := v.tailStart(); i >= t {
		return v.tail[i-t]
	}
	return v.leaf(i).kids[i&vecMask]
}

// set returns v with x at index i, which must be below n.
func (v *vec) set(i int, x Value) vec {
	out := *v
	if t := v.tailStart(); i >= t {
		out.tail, out.grow = newTail(v.tail, len(v.tail))
		out.tail[i-t] = x
		return out
	}
	out.root = setIn(v.root, v.shift, i, x)
	return out
}

// setIn returns a copy of nd, a node level bits above the leaves, with x at
// index i; only the path to i is copied.
func setIn(nd *vnode, level uint, i int, x Value) *vnode {
	out := *nd
	if level == 0 {
		out.kids[i&vecMask] = x
	} else {
		sub := (i >> level) & vecMask
		out.kids[sub] = setIn(nd.kids[sub].(*vnode), level-vecBits, i, x)
	}
	return &out
}

// newTail returns a new tail array that holds tail and has room for n
// values in all, with a growth of its own.
func newTail(tail []Value, n int) ([]Value, *growth) {
	out := make([]Value, n, vecWidth)
	copy(out, tail)
	g := &growth{}
	g.used.Store(int64(n))
	return out, g
}

// push returns v with x after its values. It takes constant time on
// average: the tail fills in place when v is the newest vec on its tail's
// array (see List.appended), and a full tail moves into the tree whole.
func (v *vec) push(x Value) vec {
	out := *v
	out.n++
	if len(v.tail) < vecWidth {
		k := len(v.tail)
		if g := v.grow; g != nil && cap(v.tail) == vecWidth && g.used.CompareAndSwap(int64(k), int64(k+1)) {
			out.tail = append(v.tail, x)
			return out
		}
		out.tail, out.grow = newTail(v.tail, k+1)
		out.tail[k] = x
		return out
	}
	leaf := &vnode{}
	copy(leaf.kids[:], v.tail)
	switch {
	case v.root == nil:
		out.root, out.shift = &vnode{}, vecBits
		out.root.kids[0] = leaf
	case v.tailStart()>>vecBits == 1<<v.shift:
		// The tree is full: a new root holds the old one and a path down
		// to the new leaf.
		out.root = &vnode{}
		out.root.kids[0] = v.root
		out.root.kids[1] = pathTo(v.shift, leaf)
		out.shift += vecBits
	default:
		out.root = pushLeaf(v.root, v.shift, v.tailStart(), leaf)
	}
	out.tail, out.grow = newTail(nil, 1)
	out.tail[0] = x
	return out
}

// pushLeaf returns a copy of nd, a node level bits above the leaves, with
// leaf added as the leaf that starts at index at.
func pushLeaf(nd *vnode, level uint, at int, leaf *vnode) *vnode {
	out := *nd
	sub := (at >> level) & vecMask
	switch child, _ := nd.kids[sub].(*vnode); {
	case level == vecBits:
		out.kids[sub] = leaf
	case child != nil:
		out.kids[sub] = pushLeaf(child, level-vecBits, at, leaf)
	default:
		out.kids[sub] = pathTo(level-vecBits, leaf)
	}
	return &out
}

// pathTo returns leaf under a chain of new nodes, level bits above it.
func pathTo(level uint, leaf *vnode) *vnode {
	if level == 0 {
		return leaf
	}
	nd := &vnode{}
	nd.kids[0] = pathTo(level-vecBits, leaf)
	return nd
}

// values returns v's values, in order, as a new slice.
func (v *vec) values() []Value {
	out := make([]Value, 0, v.n)
	for i := 0; i < v.tailStart(); i += vecWidth {
		out = append(out, v.leaf(i).kids[:]...)
	}
	return append(out, v.tail...)
}
