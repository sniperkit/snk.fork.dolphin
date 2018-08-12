/*
Sniperkit-Bot
- Status: analyzed
*/

package types

import (
	"testing"
)

func TestMustParseVersion(t *testing.T) {

	v := MustParseVersion("v1.1.1-a.b.c+build12.3")
	v1 := MustParseVersion("v1.1.1-a.b.e")
	t.Logf("%v, %v", v.version.Pre, v.version.Build)
	t.Logf("%v", v.version.Compare(v1.version))
	t.Logf("verson: %v", v)
}
