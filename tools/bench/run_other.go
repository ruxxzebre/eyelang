//go:build !unix

package main

import "errors"

var errPlatform = errors.New("the benchmarks run only on Unix: they need fork and wait4 for peak memory")

func buildLauncher(string) (string, error) { return "", errPlatform }

func runOnce(string, []string, string) (float64, int64, string, error) {
	return 0, 0, "", errPlatform
}
