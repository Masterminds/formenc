package form

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// ValidationError indicates that a field was not valid.
type ValidationError struct {
	Field, Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

// CompoundValidationError represents a list of validation errors.
type CompoundValidationError struct {
	errs []*ValidationError
}

// Len returns the count of validation errors
func (c CompoundValidationError) Len() int {
	return len(c.errs)
}

// Errors returns all validation errors
func (c CompoundValidationError) Errors() []*ValidationError {
	return c.errs
}

func (c CompoundValidationError) Error() string {
	msg := make([]string, len(c.errs))
	for _, e := range c.errs {
		msg = append(msg, e.Error())
	}
	return fmt.Sprintf("multiple validation errors occurred: %s", strings.Join(msg, "; "))
}

// Validator indicates whether a form field is valid.
type Validator func(value []string) *ValidationError

// Setter decodes a value for a field.
type Setter func(value []string) error

// FieldUnmarshaler unmarshals a specific field.
type FieldUnmarshaler func(field string, value []string) (interface{}, error)

func Unmarshal(v url.Values, o interface{}) error {
	val := reflect.ValueOf(o)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return errors.New("unmarshal requires a pointer to a receiver")
	}

	return walk(val, v)
}

func walk(val reflect.Value, v url.Values) error {
	// Loop through values, top-down specificity
	verrs := []*ValidationError{}
	for key, vals := range v {
		e := findIn(val, key, vals)
		if e == nil {
			continue
		} else if ve, ok := e.(*ValidationError); ok {
			verrs = append(verrs, ve)
			continue
		}
		return e
	}
	if len(verrs) > 0 {
		return CompoundValidationError{errs: verrs}
	}
	return nil
}

func findIn(rv reflect.Value, key string, values []string) error {
	switch reflect.Indirect(rv).Kind() {
	case reflect.Map:
		// The map must take string keys.
		if _, ok := rv.Interface().(map[string]interface{}); ok {
			return assignToMap(rv, key, values)
		}
	case reflect.Struct:
		// Look for struct field named 'key'.
		//return assignToStruct(reflect.Indirect(rv), key, values)
		return assignToStruct(rv, key, values)
	}
	return fmt.Errorf("object %s cannot be used to store values", rv.Type().Name())
}

func assignToMap(rv reflect.Value, key string, values []string) error {
	var err error
	defer func() {
		if e := recover(); e != nil {
			fmt.Printf("Failed map assignment: %v\n", e)
			// FIXME: can't modify err in recover.
			err = fmt.Errorf("failed map assignment: %s", e)
		}
	}()
	// FIXME: There must be a way to find the destination type of a map and
	// appropriately convert to it.
	switch l := len(values); {
	case l == 1:
		rv.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(values[0]))
	case l > 1:
		rv.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(values))
	}
	return err
}

func assignToStruct(rval reflect.Value, key string, values []string) error {
	ptrt := rval.Type()
	rv := reflect.Indirect(rval)
	rt := rv.Type()
	// Look for a Field on struct that matches the key name.
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := parseTag(f.Tag.Get("form"))
		if !tag.ignore && tag.name == "" {
			tag.name = f.Name
		}
		if tag.name == key {
			validator := "FormValidate" + f.Name
			setter := "FormSet" + f.Name

			// If there is a validator, call it.
			if m, ok := ptrt.MethodByName(validator); ok {
				// fmt.Printf("Validating %s against %v\n", key, m)
				if err := callFormMethod(m, rval, values); err != nil {
					return err
				}
			}

			// For assignment, if there is a setter, use it. Otherwise, do a
			// raw assignment.
			if m, ok := ptrt.MethodByName(setter); ok {
				//fmt.Printf("Setting %s with %v\n", key, m)
				return callFormMethod(m, rval, values)
			} else {
				assignToStructField(rv.FieldByName(f.Name), values)
				return nil
			}
		}
	}
	fmt.Printf("Skipped key %q", key)
	return nil
}

func callFormMethod(method reflect.Method, target reflect.Value, values []string) error {
	retvals := method.Func.Call([]reflect.Value{target, reflect.ValueOf(values)})
	if !retvals[0].IsNil() {
		// An error occurred
		return retvals[0].Interface().(error)
	}
	return nil
}

func assignToStructField(rv reflect.Value, val []string) error {
	// Basically, we need to convert from a string to the appropriate underlying
	// kind, then assign.
	switch rv.Kind() {
	case reflect.String:
		rv.Set(reflect.ValueOf(val[0]))
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		vv := "0"
		if len(val) > 0 {
			vv = val[0]
		}
		return assignToInt(rv, vv)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		vv := "0"
		if len(val) > 0 {
			vv = val[0]
		}
		return assignToUint(rv, vv)
	case reflect.Float32, reflect.Float64:
		vv := "0"
		if len(val) > 0 {
			vv = val[0]
		}
		return assignToFloat(rv, vv)
	case reflect.Bool:
		b, err := strconv.ParseBool(val[0])
		reflect.Indirect(rv).Set(reflect.ValueOf(b))
		return err
	case reflect.Slice:
		if _, ok := rv.Interface().([]string); ok {
			reflect.Indirect(rv).Set(reflect.ValueOf(val))
			return nil
		}
		return fmt.Errorf("Only string slices are supported.")
	default:
		return fmt.Errorf("Unsupported kind")
	}
}

func assignToInt(rv reflect.Value, val string) error {
	rvv := reflect.Indirect(rv)
	if !rvv.CanSet() {
		return fmt.Errorf("cannot set %q (%s)", rv.Type().Name(), rv.Kind().String())
	}
	ival, err := strconv.ParseInt(val, 0, 0)
	if err != nil {
		return err
	}
	rvv.SetInt(ival)
	return nil
}
func assignToUint(rv reflect.Value, val string) error {
	rvv := reflect.Indirect(rv)
	if !rvv.CanSet() {
		return fmt.Errorf("cannot set %q (%s)", rv.Type().Name(), rv.Kind().String())
	}
	ival, err := strconv.ParseUint(val, 0, 0)
	if err != nil {
		return err
	}
	rvv.SetUint(ival)
	return nil
}
func assignToFloat(rv reflect.Value, val string) error {
	rvv := reflect.Indirect(rv)
	if !rvv.CanSet() {
		return fmt.Errorf("cannot set %q (%s)", rv.Type().Name(), rv.Kind().String())
	}
	ival, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return err
	}
	rvv.SetFloat(ival)
	return nil
}

func parseTag(str string) *tag {
	parts := strings.Split(str, ",")
	if len(parts) == 1 && parts[0] == "" {
		return &tag{}
	}
	t := &tag{}
	switch n := parts[0]; n {
	case "+":
		t.group = true
	case "-":
		t.ignore = true
	default:
		t.name = n
	}

	for _, p := range parts[1:] {
		switch {
		case p == "omitempty":
			t.omit = true
		case strings.HasPrefix(p, "prefix="):
			t.prefix = strings.TrimPrefix(p, "prefix=")
		case strings.HasPrefix(p, "suffix="):
			t.suffix = strings.TrimPrefix(p, "suffix=")
		}
	}
	return t
}

// tag represents a 'form' tag.
//
//	Name string `form:name`
//	Date time.Time `form:date,omitempty`
//	Address *Address `form:+,omitempty,prefix=addr_
type tag struct {
	name           string
	prefix, suffix string //prefix=, suffix=
	omit           bool   // omitempty
	ignore         bool   // -
	group          bool   // +
	validator      Validator
	unmarshaler    FieldUnmarshaler
}
