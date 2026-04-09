package main

// version is populated at build time via -ldflags "-X main.version=…".
// Default "dev" when built without ldflags.
var version = "dev"
