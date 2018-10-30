package error

import (
	"strings"
	"sync"
)

type MultiError struct {
	errors []error
	f      formatFunc
}

type formatFunc func([]error) string

func defaultErrorToString(errors []error) string {
	if len(errors) == 0 {
		return ""
	}

	strBuilder, _ := rawStringBuilderPool.Get().(*strings.Builder)
	defer rawStringBuilderPool.Put(strBuilder)
	strBuilder.Reset()

	for index, v := range errors {
		if index != 0 {
			strBuilder.WriteString("; ")
		}
		strBuilder.WriteString(v.Error())
	}
	return strBuilder.String()
}

func (m *MultiError) Error() string {
	if m == nil {
		return ""
	}
	if len(m.errors) == 0 {
		return ""
	}
	if m.f == nil {
		m.f = defaultErrorToString
	}
	return m.f(m.errors)
}

func (m *MultiError) ErrorOrNil() error {
	if m == nil {
		return nil
	}
	if len(m.errors) == 0 {
		return nil
	}
	return m
}

func (m *MultiError) Append(err error) *MultiError {
	if m == nil {
		m = new(MultiError)
	}
	if err != nil {
		m.errors = append(m.errors, err)
	}
	return m
}

func (m *MultiError) SetFormatFunc(fn formatFunc) *MultiError {
	if m == nil {
		m = new(MultiError)
	}
	m.f = fn
	return m
}

var rawStringBuilderPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}
