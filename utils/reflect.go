package utils

import "reflect"

// 反射参数，一些通过反射调用的方法，参数需要转拒成 reflect.Value
func ReflectArguments(args ...interface{}) []reflect.Value {
	passedArguments := make([]reflect.Value, 0, len(args))
	for _, arg := range args {
		passedArguments = append(passedArguments, reflect.ValueOf(arg))
	}
	return passedArguments
}

// 反射结果，一些通过反射调用的方法，返回的结果需要转换成正常的值
func ReflectResults(results ...reflect.Value) []interface{} {
	passedResults := make([]interface{}, 0, len(results))
	for _, result := range results {
		passedResults = append(passedResults, result.Interface())
	}
	return passedResults
}
