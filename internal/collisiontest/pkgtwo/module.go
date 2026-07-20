// Package oauth is a test helper whose package name intentionally collides
// with internal/collisiontest/pkgtwo so that both declare a Module type whose
// reflect.Type.String() renders identically (e.g. "*oauth.Module").
package oauth

import "flamingo.me/dingo"

// Configured counts how often Module.Configure was called.
var Configured int

// Module is a dingo Module used to reproduce cross-package name collisions in
// moduleIdentity tests.
type Module struct{}

// Configure records that it ran so a test can assert the module was not
// deduplicated away.
func (*Module) Configure(injector *dingo.Injector) {
	Configured++
}
