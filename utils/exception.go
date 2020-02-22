package utils

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"reflect"
	"runtime"
	"strings"
)

func PrintStackTrace(err interface{}) {
	plog.WithFields(logrus.Fields{
		"errMsg": err,
	}).Error("")

	i := 0
	funcName, file, line, ok := runtime.Caller(i)
	for ok {
		name := runtime.FuncForPC(funcName).Name()
		if !strings.HasPrefix(name, "runtime.") && !strings.Contains(name, "PrintStackTrace") {
			plog.Error(fmt.Sprintf("frame %v:[func:%v,file:%v,line:%v]", i, runtime.FuncForPC(funcName).Name(), file, line))
		}
		i++
		funcName, file, line, ok = runtime.Caller(i)
	}
}

type CatchHandler interface {
	Catch(err interface{}, handler func(err interface{})) CatchHandler
	CatchAll(handler func(err interface{})) FinalHandler
	FinalHandler
}

type FinalHandler interface {
	Finally(handlers ...func())
}

func Try(f func()) CatchHandler {
	t := &catchHandler{}
	defer func() {
		defer func() {
			r := recover()
			if r != nil {
				t.err = r
			}
		}()
		f()
	}()
	return t
}

type catchHandler struct {
	err      interface{}
	hasCatch bool
}

//有两个作用：一个是判断是否已捕捉异常，另一个是否发生了异常。如果返回false则代表没有异常，或异常已被捕捉。
func (t *catchHandler) RequireCatch() bool {
	if t.hasCatch { //如果已经执行了catch块，就直接判断不执行了
		return false
	}
	if t.err == nil { //如果异常为空，则判断不执行
		return false
	}
	return true
}

func (t *catchHandler) Catch(err interface{}, handler func(err interface{})) CatchHandler {
	if !t.RequireCatch() {
		return t
	}

	if reflect.TypeOf(err) == reflect.TypeOf(t.err) {
		handler(t.err)
		t.hasCatch = true
	}
	return t
}

func (t *catchHandler) CatchAll(handler func(err interface{})) FinalHandler {
	//返回同一个对像，接收的接口不一样，CatchAll后只能调用 Finally
	if !t.RequireCatch() {
		return t
	}
	handler(t.err)
	t.hasCatch = true
	return t
}

func (t *catchHandler) Finally(handlers ...func()) {
	for _, handler := range handlers {
		defer handler()
	}
	err := t.err

	//异常未捕捉，抛出异常
	if err != nil && !t.hasCatch {
		panic(err)
	}
}
