package tcabcireadgoclient

import (
	"errors"
	"reflect"
)

func StringOR(values ...string) string {
	for i := 0; i < len(values); i++ {
		if values[i] != "" {
			return values[i]
		}
	}

	return ""
}

func InArray(val interface{}, array interface{}) (exists bool, index int, err error) {
	exists = false
	index = -1

	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				index = i
				exists = true
				return
			}
		}
	default:
		return false, 0, errors.New("invalid value. expected value type is slice")
	}

	return
}
