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
	// Usually, you get values from a `*http.Request`.
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
