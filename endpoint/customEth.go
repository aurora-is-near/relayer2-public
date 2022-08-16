package endpoint

import (
	"aurora-relayer-go-common/endpoint"
	"errors"
	"fmt"
)

type CustomEth struct {
	*endpoint.Endpoint
}

func NewCustomEth(endpoint *endpoint.Endpoint) *CustomEth {
	return &CustomEth{endpoint}
}

// Optional1 A sample method to show the usage of optional values via "eth_optional1" signature
func (e *CustomEth) Optional1(o1 *int, o2 *int) string {
	var tmp string
	if o1 == nil {
		tmp = "Optional Param [0] is nil,"
	} else {
		tmp = fmt.Sprintf("Optional Param [0] is %d,", *o1)
	}

	if o2 == nil {
		tmp = tmp + " Optional Param [1] is nil"
	} else {
		tmp = tmp + fmt.Sprintf(" Optional Param [1] is %d", *o2)
	}
	return tmp
}

// Default1 A sample method to show the usage of default values via "eth_default1" signature
func (e *CustomEth) Default1(arg1 *interface{}, arg2 *interface{}) (string, error) {
	o1 := "LATEST"
	m1 := false

	strCounter := 0
	boolCounter := 0

	if arg1 != nil {
		a1 := *arg1

		switch t1 := a1.(type) {
		case string:
			o1 = t1
			strCounter++
		case bool:
			m1 = t1
			boolCounter++
		default:
			return "", errors.New("invalid argument type")
		}
	}

	if arg2 != nil {
		a2 := *arg2
		switch t2 := a2.(type) {
		case string:
			o1 = t2
			strCounter++
		case bool:
			m1 = t2
			boolCounter++
		default:
			return "", errors.New("invalid argument type")
		}
	}

	if strCounter > 1 || boolCounter > 1 {
		return "", errors.New("invalid argument type")
	}

	return fmt.Sprintf("Optional Param [0] is %s, and Optional Param [1] is %t", o1, m1), nil
}
