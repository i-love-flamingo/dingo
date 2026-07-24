package dingo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"flamingo.me/dingo"
	pkgone "flamingo.me/dingo/internal/collisiontest/pkgone"
	pkgtwo "flamingo.me/dingo/internal/collisiontest/pkgtwo"
)

// TestNewInjector_SameShortNameDifferentPackages guards against the
// moduleIdentity() collision fixed in DIGIHUB-374210: two Module types that
// live in different packages but share the same package name render
// identically via reflect.Type.String() ("*oauth.Module"). Keying modules by
// that string silently dropped one of them, so its Configure never ran. Both
// modules must be initialized.
func TestNewInjector_SameShortNameDifferentPackages(t *testing.T) {
	t.Parallel()

	pkgone.Configured = 0
	pkgtwo.Configured = 0

	injector, err := dingo.NewInjector(new(pkgone.Module), new(pkgtwo.Module))

	require.NoError(t, err)
	require.NotNil(t, injector)
	assert.Equal(t, 1, pkgone.Configured, "pkgone.Module must not be deduplicated away")
	assert.Equal(t, 1, pkgtwo.Configured, "pkgtwo.Module must not be deduplicated away")
}
