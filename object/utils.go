package object

func CopyArray(arr []Object) []Object {
	newArr := make([]Object, len(arr), len(arr))
	copy(newArr, arr)
	return newArr
}
