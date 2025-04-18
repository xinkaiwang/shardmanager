package klogging

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// SimpleFormatter implements the logrus.Formatter interface
type SimpleFormatter struct {
}

func NewSimpleFormatter() logrus.Formatter {
	return &SimpleFormatter{}
}

// implements the logrus.Formatter interface
func (f *SimpleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var sb strings.Builder
	dict := entry.Data
	sb.WriteString(entry.Time.Format("2006-01-02 15:04:05.000"))
	sb.WriteString(" ")
	sb.WriteString(strings.ToUpper(entry.Level.String()))
	sb.WriteString(" ")
	// event
	if event, ok := dict["event"]; ok {
		delete(dict, "event")
		sb.WriteString("event=")
		sb.WriteString(event.(string))
		sb.WriteString(" ")
	}
	// msg
	sb.WriteString("msg=")
	sb.WriteString(AddingAdditionalQuotes(entry.Message))
	sb.WriteString(" ")

	for k, v := range dict {
		if k == "time" || k == "level" {
			continue
		}
		sb.WriteString(k)
		sb.WriteString("=")

		// Use reflection value to handle various underlying types generically
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.String:
			sb.WriteString(AddingAdditionalQuotes(rv.String()))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			sb.WriteString(strconv.FormatInt(rv.Int(), 10))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			sb.WriteString(strconv.FormatUint(rv.Uint(), 10))
		case reflect.Float32, reflect.Float64:
			sb.WriteString(strconv.FormatFloat(rv.Float(), 'f', -1, 64))
		case reflect.Bool:
			sb.WriteString(strconv.FormatBool(rv.Bool()))
		case reflect.Map, reflect.Slice, reflect.Struct, reflect.Ptr, reflect.Interface, reflect.Array:
			// For complex types, use default Go representation (or potentially JSON)
			// Using %v might be sufficient for simple cases.
			// Consider using json.Marshal for more complex structures if needed.
			sb.WriteString(fmt.Sprintf("%v", v))
		default:
			// Fallback for any other type
			sb.WriteString(fmt.Sprintf("%v", v))
		}
		sb.WriteString(" ")
	}
	sb.WriteString("\n")
	return []byte(sb.String()), nil
}

func AddingAdditionalQuotes(v string) string {
	// remove all the \n
	v = strings.ReplaceAll(v, "\n", "")
	// Add quotes if the string contains spaces or is empty
	if v == "" || strings.Contains(v, " ") {
		// Escape existing single quotes if necessary before adding surrounding ones
		return "'" + strings.ReplaceAll(v, "'", "\\'") + "'"
	}
	return v
}
