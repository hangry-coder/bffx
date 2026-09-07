package testing

import (
	"fmt"
	"reflect"
)

type Expectation struct {
	actual any
	failed bool
}

func Expect(actual any) *Expectation {
	return &Expectation{actual: actual}
}

func (e *Expectation) ToEqual(expected any) {
	if !reflect.DeepEqual(e.actual, expected) {
		e.fail(fmt.Sprintf("Expected %v to equal %v", e.actual, expected))
	}
}

func (e *Expectation) ToNotEqual(expected any) {
	if reflect.DeepEqual(e.actual, expected) {
		e.fail(fmt.Sprintf("Expected %v to NOT equal %v", e.actual, expected))
	}
}

func (e *Expectation) ToBeTrue() {
	if val, ok := e.actual.(bool); !ok || !val {
		e.fail(fmt.Sprintf("Expected %v to be true", e.actual))
	}
}

func (e *Expectation) ToBeEmpty() {
	v := reflect.ValueOf(e.actual)
	if v.Len() > 0 {
		e.fail(fmt.Sprintf("Expected %v to be empty", e.actual))
	}
}

func (e *Expectation) fail(msg string) {
	if currentT != nil {
		currentT.Errorf("    FAILED: %s", msg)
	} else {
		fmt.Printf("    FAILED: %s\n", msg)
	}
}
