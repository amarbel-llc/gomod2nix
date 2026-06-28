package main

// #cgo pkg-config: zlib
// #include <zlib.h>
import "C"

import "fmt"

// version and commit are injected via -ldflags by buildGoApplication's
// versionLdflags; declaring them keeps the linker from warning about missing
// symbols.
var (
	version string
	commit  string
)

func main() {
	// greeting is produced by the code-generation step (greeting_gen.go). If
	// that step did not run, this file fails to compile — which is the point:
	// it proves the nativeBuildInputs generator executed during the build.
	//
	// C.zlibVersion() exercises the cgo path (pkg-config: zlib resolves via the
	// pkg-config nativeBuildInput against the zlib buildInput).
	fmt.Println(greeting, C.GoString(C.zlibVersion()), version, commit)
}
