//go:build egoonly

package main

// egoOnly marks the Ego-native build: same 49 tools, but defaults tuned for the
// official ego-browser skill (no accessibility tree unless asked, follow new tabs).
var egoOnly = true
