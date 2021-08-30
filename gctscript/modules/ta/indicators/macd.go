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

// MACDModule MACD indicator commands
var MACDModule = map[string]objects.Object{
	"calculate": &objects.UserFunction{Name: "calculate", Value: macd},
}

// MovingAverageConvergenceDivergence is the string constant
const MovingAverageConvergenceDivergence = "Moving Average Convergence Divergence"

// MACD defines a custom Moving Average Convergence Divergence tengo indicator
// object type
type MACD struct {
	objects.Array
	Period, PeriodSlow, PeriodFast int
}

// TypeName returns the name of the custom type.
func (o *MACD) TypeName() string {
	return MovingAverageConvergenceDivergence
}

func macd(args ...objects.Object) (objects.Object, error) {
	if len(args) != 4 {
		return nil, objects.ErrWrongNumArguments
	}

	r := new(MACD)
	if validator.IsTestExecution.Load() == true {
		return r, nil
	}

	ohlcvInput := objects.ToInterface(args[0])
	ohlcvInputData, valid := ohlcvInput.([]interface{})
	if !valid {
		return nil, fmt.Errorf(modules.ErrParameterConvertFailed, OHLCV)
	}

	var ohlcvClose []float64
	var allErrors []string
	for x := range ohlcvInputData {
		t := ohlcvInputData[x].([]interface{})
		value, err := toFloat64(t[4])
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
		ohlcvClose = append(ohlcvClose, value)
	}

	inFastPeriod, ok := objects.ToInt(args[1])
	if !ok {
		allErrors = append(allErrors, fmt.Sprintf(modules.ErrParameterConvertFailed, inFastPeriod))
	}

	inSlowPeriod, ok := objects.ToInt(args[2])
	if !ok {
		allErrors = append(allErrors, fmt.Sprintf(modules.ErrParameterConvertFailed, inSlowPeriod))
	}

	inTimePeriod, ok := objects.ToInt(args[3])
	if !ok {
		allErrors = append(allErrors, fmt.Sprintf(modules.ErrParameterConvertFailed, inTimePeriod))
	}

	if len(allErrors) > 0 {
		return nil, errors.New(strings.Join(allErrors, ", "))
	}

	r.Period = inTimePeriod
	r.PeriodFast = inFastPeriod
	r.PeriodSlow = inSlowPeriod

	macd, macdSignal, macdHist := indicators.MACD(ohlcvClose, inFastPeriod, inSlowPeriod, inTimePeriod)
	for x := range macdHist {
		tempMACD := &objects.Array{}
		tempMACD.Value = append(tempMACD.Value, &objects.Float{Value: math.Round(macdHist[x]*100) / 100})
		if macd != nil {
			tempMACD.Value = append(tempMACD.Value, &objects.Float{Value: math.Round(macd[x]*100) / 100})
		}
		if macdSignal != nil {
			tempMACD.Value = append(tempMACD.Value, &objects.Float{Value: math.Round(macdSignal[x]*100) / 100})
		}
		r.Value = append(r.Value, tempMACD)
	}

	return r, nil
}
