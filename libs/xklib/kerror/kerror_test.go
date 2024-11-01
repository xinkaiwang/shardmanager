package kerror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKerrorBasic(t *testing.T) {
	e1 := Create("Type1", "error happened")
	expected := "Type1: error happened"
	// fmt.Println(e1.Stack)
	assert.Regexp(t, expected, e1.Error())
}

func TestKerrorWithStack(t *testing.T) {
	e1 := Create("Type1", "error happened")
	expected := "Type1: error happened, stack=github.com/xinkaiwang/shardmanager/libs/xklib/kerror.TestKerrorWithStack"
	// log.Println(e1.Error())
	assert.Regexp(t, expected, e1.FullString())
}

func TestKerrorWithFields(t *testing.T) {
	e1 := Create("Type1", "error happened").With("elapsedMs", 120).With("tracingId", "1234fe27").With("key", nil).With("val", []byte("test"))
	// Type1: error happened, elapsedMs=120, tracingId=1234fe27, key=<nil>, val=74657374
	str := e1.Error()
	// fmt.Println(str)
	assert.Regexp(t, "elapsedMs=120,", str)
	assert.Regexp(t, "tracingId=1234fe27,", str)
	assert.Regexp(t, "key=<nil>,", str)
	assert.Regexp(t, "val=74657374", str)
}

func TestNerrorCausedByKerror(t *testing.T) {
	e1 := Create("Type1", "error Type1 happened")
	e2 := Wrap(e1, "Type2", "another level", true /* needStack */).With("elapsedMs", 200).With("tracingId", "1234fedc")
	expected := "Type2: another level, elapsedMs=200, tracingId=1234fedc;\n Caused by: Type1: error Type1 happened, stack=github.com/xinkaiwang/shardmanager/libs/xklib/kerror.TestNerrorCausedByKerror"
	// fmt.Println(e2.FullString())
	assert.Regexp(t, expected, e2.FullString())
}

func TestNerrorCausedByError(t *testing.T) {
	e1 := errors.New("hello")
	e2 := Wrap(e1, "Type2", "another level", true /* needStack */).WithErrorCode(EC_INVALID_PARAMETER)
	// log.Println(e2.FullString())
	assert.Regexp(t, "^Type2: another level", e2.FullString())
	assert.Regexp(t, "Caused by: hello", e2.FullString())
	assert.Regexp(t, "^Type2: another level", e2.ShortString())
	assert.Regexp(t, "^Type2: another level", e2.String())
	assert.Regexp(t, "hello", e2.CausedByString())
}
