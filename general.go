//go:build !egoonly

package main

// egoOnly is false for the general build, which also targets non-Ego browsers
// (Edge/CDP) and returns the accessibility tree by default.
var egoOnly = false
