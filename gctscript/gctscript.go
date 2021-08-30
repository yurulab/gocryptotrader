package gctscript

import (
	"github.com/yurulab/gocryptotrader/gctscript/modules"
	"github.com/yurulab/gocryptotrader/gctscript/wrappers/gct"
)

// Setup configures the wrapper interface to use
func Setup() {
	modules.SetModuleWrapper(gct.Setup())
}
