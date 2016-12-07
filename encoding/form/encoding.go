package form

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

var LogDebug = false

// ValidationError indicates that a field was not valid.
type ValidationError struct {
	Field, Message string
}

// Invalid constructs a ValidationError.
func Invalid(field, msg string, v ...interface{}) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(msg, v...)}
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
	msg := []string{}
	for _, e := range c.errs {
		msg = append(msg, e.Error())
	}
	return fmt.Sprintf("validation errors occurred: %s", strings.Join(msg, "; "))
}

// Validator indicates whether a form field is valid.
type Validator func(value []string) *ValidationError

// Setter decodes a value for a field.
type Setter func(value []string) error

// FieldUnmarshaler unmarshals a specific field.
type FieldUnmarshaler func(field string, value []string) (interface{}, error)

// Unmarshal unmarshals values into the given interface{}.
//
// This walks the values and copies each value into the matching field on the
// interface. It is recommended that o be a pointer to a struct.
//
// Structs may be annotated with tags:
//
//	type Person struct {
//		First string `form:"first_name"`
//		Last string `form:"last_name"`
//	}
//
// Additionally, if a field has a matching Validator or Setter, that function
// will also be called. Validators and Setters are matched based on name.
// For example, given the First field above, the validators and setters would
// be:
//
//	func(p *Person) FormValidateFirst(v []string) *form.ValidationError {}
//	func(p *Person) FormSetFirst(v []string) error {}
//
// A Validator should not alter v or store any part of v.
//
// A Setter may set the field. If it does not, the field will remain unset.
//
// If a Validator fails, the field will not be set, but processing will continue.
// If a Setter fails, the field will not be set, and unmarshaling will be halter.
func Unmarshal(v url.Values, o interface{}) error {
	val := reflect.ValueOf(o)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return errors.New("unmarshal requires a pointer to a receiver")
	}

	return walk(val, v)
}

// Tags returns the tags on all of the fields on the struct that have 'form' annotations.
func Tags(o interface{}) []*Tag {
	tt := reflect.Indirect(reflect.ValueOf(o)).Type()
	if tt.Kind() != reflect.Struct {
		return []*Tag{}
	}
	tags := []*Tag{}

	// Look for a Field on struct that matches the key name.
	for i := 0; i < tt.NumField(); i++ {
		f := tt.Field(i)
		tag := parseTag(f.Tag.Get("form"))
		if !tag.Ignore && tag.Name == "" {
			tag.Name = f.Name
		}
		tags = append(tags, tag)
	}
	return tags
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
		if !tag.Ignore && tag.Name == "" {
			tag.Name = f.Name
		}
		if tag.Name == key {
			validator := "FormValidate" + f.Name
			setter := "FormSet" + f.Name

			// If there is a validator, call it.
			if m, ok := ptrt.MethodByName(validator); ok {
				if LogDebug {
					log.Printf("Validating %s against %v\n", key, m)
				}
				if err := callFormMethod(m, rval, values); err != nil {
					if LogDebug {
						log.Printf("Validation of %s=%v failed: %s", key, values, err)
					}
					return err
				}
			}

			// For assignment, if there is a setter, use it. Otherwise, do a
			// raw assignment.
			if m, ok := ptrt.MethodByName(setter); ok {
				if LogDebug {
					log.Printf("Setting %s with %v\n", key, m)
				}
				return callFormMethod(m, rval, values)
			} else {
				if LogDebug {
					log.Printf("Assigning %s value %v", key, values)
				}
				err := assignToStructField(rv.FieldByName(f.Name), values)
				if LogDebug && err != nil {
					log.Printf("Error assigning %s value %v: %s", key, values, err)
				}
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

// empty returns true if v's len is 0, or v's len is 1 and v[0]'s len is 0
func empty(v []string) bool {
	if len(v) == 0 {
		return true
	}
	return len(v) == 1 && len(v[0]) == 0
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
		if !empty(val) {
			vv = val[0]
		}
		return assignToInt(rv, vv)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		vv := "0"
		if !empty(val) {
			vv = val[0]
		}
		return assignToUint(rv, vv)
	case reflect.Float32, reflect.Float64:
		vv := "0"
		if !empty(val) {
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

func parseTag(str string) *Tag {
	parts := strings.Split(str, ",")
	if len(parts) == 1 && parts[0] == "" {
		return &Tag{}
	}
	t := &Tag{}
	switch n := parts[0]; n {
	case "+":
		t.Group = true
	case "-":
		t.Ignore = true
	default:
		t.Name = n
	}

	for _, p := range parts[1:] {
		switch {
		case p == "omitempty":
			t.Omit = true
		case strings.HasPrefix(p, "prefix="):
			t.Prefix = strings.TrimPrefix(p, "prefix=")
		case strings.HasPrefix(p, "suffix="):
			t.Suffix = strings.TrimPrefix(p, "suffix=")
		}
	}
	return t
}

// tag represents a 'form' tag.
//
//	Name string `form:name`
//	Date time.Time `form:date,omitempty`
//	Address *Address `form:+,omitempty,prefix=addr_
type Tag struct {
	Name           string
	Prefix, Suffix string //prefix=, suffix=
	Omit           bool   // omitempty
	Ignore         bool   // -
	Group          bool   // +
	validator      Validator
	unmarshaler    FieldUnmarshaler
}
