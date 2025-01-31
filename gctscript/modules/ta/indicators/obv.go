package indicators

import (
	"errors"
	"fmt"
	"math"
	"strings"

	objects "github.com/d5/tengo/v2"
	"github.com/thrasher-corp/gct-ta/indicators"
	"github.com/yurulab/gocryptotrader/gctscript/modules"
	"github.com/yurulab/gocryptotrader/gctscript/wrappers/validator"
)

// ObvModule volume indicator commands
var ObvModule = map[string]objects.Object{
	"calculate": &objects.UserFunction{Name: "calculate", Value: obv},
}

// OnBalanceVolume is the string constant
const OnBalanceVolume = "On Balance Volume"

// OBV defines a custom On Balance Volume tengo indicator object type
type OBV struct {
	objects.Array
}

// TypeName returns the name of the custom type.
func (o *OBV) TypeName() string {
	return OnBalanceVolume
}

func obv(args ...objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.ErrWrongNumArguments
	}

	r := new(OBV)
	if validator.IsTestExecution.Load() == true {
		return r, nil
	}

	ohlcvInput := objects.ToInterface(args[0])
	ohlcvInputData, valid := ohlcvInput.([]interface{})
	if !valid {
		return nil, fmt.Errorf(modules.ErrParameterConvertFailed, OHLCV)
	}

	ohlcvData := make([][]float64, 6)
	var allErrors []string
	for x := range ohlcvInputData {
		t := ohlcvInputData[x].([]interface{})
		value, err := toFloat64(t[2])
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
		ohlcvData[2] = append(ohlcvData[2], value)

		value, err = toFloat64(t[3])
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
		ohlcvData[3] = append(ohlcvData[3], value)

		value, err = toFloat64(t[4])
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
		ohlcvData[4] = append(ohlcvData[4], value)

		value, err = toFloat64(t[5])
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
		ohlcvData[5] = append(ohlcvData[5], value)
	}

	if len(allErrors) > 0 {
		return nil, errors.New(strings.Join(allErrors, ", "))
	}

	ret := indicators.OBV(ohlcvData[4], ohlcvData[5])
	for x := range ret {
		temp := &objects.Float{Value: math.Round(ret[x]*100) / 100}
		r.Value = append(r.Value, temp)
	}
	return r, nil
}
