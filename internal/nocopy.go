package internal

// NoCopy may be added to structs which must not be copied after the first use.
// This is a compile-time safety mechanism to prevent accidental copying of structs
// that contain resources that should not be duplicated.
//
// See https://golang.org/issues/8005#issuecomment-190753527 for details.
//
// Note that it must not be embedded, due to the Lock and Unlock methods.
type NoCopy struct{}

// Lock is a no-op method used by the -copylocks checker from `go vet`.
// This method helps the static analyzer detect potential copying issues.
func (*NoCopy) Lock() {}

// Unlock is a no-op method used by the -copylocks checker from `go vet`.
// This method helps the static analyzer detect potential copying issues.
func (*NoCopy) Unlock() {}
