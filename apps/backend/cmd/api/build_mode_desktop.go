//go:build openad_desktop

package main

// desktopBuild is enabled only for the shipped Windows desktop API binary.
// That binary must never honor an ambient host-binding environment variable.
const desktopBuild = true
