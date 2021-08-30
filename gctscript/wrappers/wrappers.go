package wrappers

import (
	"github.com/yurulab/gocryptotrader/gctscript/modules"
	"github.com/yurulab/gocryptotrader/gctscript/wrappers/validator"
)

// GetWrapper returns the instance of each wrapper to use
func GetWrapper() modules.GCT {
	if validator.IsTestExecution.Load() == true {
		return validator.Wrapper{}
	}
	return modules.Wrapper
}
