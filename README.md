# formenc: Easily unmarshal forms

This library provides unmarshaling support for HTTP form
data. It is loosely modeled after the Go encoders.

**This is pre-release software. Not all of its features are currently
implemented.**

Right now, this library can unmarshal forms into flat structs.
Additionally, it supports `[]string` as a value on a struct. More will
be added as we get time. If you'd like to quickly support another type,
use the `FormSetFIELD` pattern described below.

## Usage

This library provides unmarshal functions for form data.

```go
package main

import (
	"fmt"
	"net/url"

	"github.com/Masterminds/formenc/encoding/form"
)

// This is a struct set up for form unmarshaling.
type Person struct {
	// The annotation indicates how the HTTP values are inserted into the struct.
	FirstName string `form:"first_name"`
	LastName  string `form:"last_name"`
	Age       int    `form:"age"`
}

func main() {
	// Usually, you get values from an `*http.Request`.
	v := url.Values{}
	for key, val := range map[string]string{
		"first_name": "Batty",
		"last_name":  "Penderwick",
		"age":        "11",
	} {
		v.Set(key, val)
	}

	// We want to unmarshal the form data into struct.
	person := &Person{}
	if err := form.Unmarshal(v, person); err != nil {
		panic(err)
	}

	fmt.Printf("%s %s is %d years old\n", person.FirstName, person.LastName, person.Age)
}
```


## Struct Tags for 'encoding/form'

The format of an `encoding/form` struct tag is:

```
form="NAME,omitempty,prefix=PREFIX,suffix=SUFFIX"
```

- `NAME` may be:
  - `-`: Field is skipped
  - `+`: Field represents a group (currently unused)
  - A name, which is the name of the form field (`name=` or, in rare
    cases, `id=`)
- `omitempty` causes the field to be left uninitialized if no value is
  present.
- `prefix` allows a group prefix to be specified (currently unused)
- `suffix` allows a group suffix to be specified (currently unused)

## Validator and Setter Functions

During the decoding phase, a struct has two opportunities to interject
functionality into the decoding process:

- Validation: Functions following a specific naming convention can
  _validate_ submitted values before they are inserted into the struct.
  This provides an oppotunity to validate data and return a complete
  error message with all validation errors.
- Setting: Functions following the setter naming convention can _set_
  the value on a field. If an error occurs while setting, the decoding
  process is stopped and immediately returned.

### Validator

To declare a validator as a method on a struct, use a method with the
following signature:

```go
// Validator indicates whether a form field is valid.
type Validator func(value []string) *ValidationError
```

It must match the following naming convention: `FormValidateNAME`.

Above, `FIELD` is the name of the field on the struct, and `values` is
the array of values received in the `url.Values` object.

A validation error should be returned if the `values` is not well
formed.

_A validator should validate `values`, but not manipulate or store
`values`._

### Setter

To declare a form setter as a struct method, use this signature:

```go

// Setter decodes a value for a field.
type Setter func(value []string) error
```

It must match the following naming convention: `FormSetNAME`.

A setter _is responsible for setting the `value` on the struct_. If this
function returns a non-nil error, the form processing will immediately
return in a failed state.
