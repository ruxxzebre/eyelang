// Package benchmarks measures eye on inputs larger than its tests use: eye
// fmt on copies of the programs under testdata, and eye check and eye build
// on a generated program of any size. The inputs are generated into
// temporary directories, never committed. README.md says how to run and
// compare them.
package benchmarks
