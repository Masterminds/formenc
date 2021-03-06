package form

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type ExampleAddress struct {
	Street string `form:"street"`
	City   string `form:"city"`
	State  string `form:"state"`
	Zip    string `form:"zip"`
}

type ExampleForm struct {
	// Retrieve these files from the form.
	FirstName string `form:"first_name"`
	LastName  string `form:"last_name"`
	// Retrieve these fields, but Omit empty address
	HomeAddress *ExampleAddress `form:"+,omitempty"`
	// This uses Prefix to modify the field name on the address. So 'street'
	// becomes 'mail_street'.
	MailingAddress *ExampleAddress `form:"+,omitempty,prefix=mail_"`
	// Ignore this field
	Processed bool `form:"-"`
}

func ExampleUnmarshal() {
	v := url.Values{}
	v.Set("first_name", "Matt")
	v.Set("street", "1234 Tape Dr")
	v.Set("mail_street", "4321 Disk Dr")

	ef := &ExampleForm{}
	if err := Unmarshal(v, ef); err != nil {
		panic(err)
	}

	fmt.Printf("Form: %v", v)
}

/*
func TestUnmarshal(t *testing.T) {
	v := url.Values{}
	v.Set("first_name", "Matt")
	v.Set("street", "1234 Tape Dr")
	v.Set("mail_street", "4321 Disk Dr")

	ef := &ExampleForm{}
	if err := Unmarshal(v, ef); err != nil {
		t.Fatal(err)
	}

	if ef.FirstName != "Matt" {
		t.Errorf("Expected Matt, got %q", ef.FirstName)
	}

	if ef.LastName != "" {
		t.Errorf("Expected empty string, got %q", ef.LastName)
	}

	if ef.StreetAddress.Street != "1234 Tape Dr" {
		t.Errorf("Unexpected mailing address: %q", ef.MailingAddress.Street)
	}

	if ef.MailingAddress.Street != "4321 Disk Dr" {
		t.Errorf("Unexpected mailing address: %q", ef.MailingAddress.Street)
	}
}
*/

func TestParseTag(t *testing.T) {
	tests := []struct {
		name   string
		tag    string
		expect Tag
	}{
		{
			name:   "name only",
			tag:    "first_name",
			expect: Tag{Name: "first_name"},
		},
		{
			name:   "name, omitempty",
			tag:    "first_name,omitempty",
			expect: Tag{Name: "first_name", Omit: true},
		},
		{
			name:   "ignore",
			tag:    "-",
			expect: Tag{Ignore: true},
		},
		{
			name:   "christmas tree",
			tag:    "name,prefix=pre_,suffix=suf_,omitempty",
			expect: Tag{Name: "name", Prefix: "pre_", Suffix: "suf_", Omit: true},
		},
		{
			name:   "group",
			tag:    "+,prefix=pre_,suffix=suf_,omitempty",
			expect: Tag{Group: true, Prefix: "pre_", Suffix: "suf_", Omit: true},
		},
	}

	for _, tt := range tests {
		got := parseTag(tt.tag)
		expect := tt.expect
		if got.Name != expect.Name {
			t.Errorf("%s expected %q, got %q", tt.name, expect.Name, got.Name)
		}
		if got.Prefix != expect.Prefix {
			t.Errorf("%s expected %q, got %q", tt.name, expect.Prefix, got.Prefix)
		}
		if got.Suffix != expect.Suffix {
			t.Errorf("%s expected %q, got %q", tt.name, expect.Suffix, got.Suffix)
		}
		if got.Group != expect.Group {
			t.Errorf("%s expected %t got %t", tt.name, expect.Group, got.Group)
		}
		if got.Ignore != expect.Ignore {
			t.Errorf("%s expected %t got %t", tt.name, expect.Ignore, got.Ignore)
		}
		if got.Omit != expect.Omit {
			t.Errorf("%s expected %t got %t", tt.name, expect.Omit, got.Omit)
		}
	}
}

func TestTags(t *testing.T) {
	var fa FixtureAddress
	tags := Tags(fa)
	if len(tags) != 4 {
		t.Fatalf("Expected4, got %d", len(tags))
	}
}

func TestAssignToMap(t *testing.T) {
	m := map[string]string{}
	mv := reflect.ValueOf(m)

	if err := assignToMap(mv, "test", []string{"first"}); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	if m["test"] != "first" {
		t.Errorf("Expected test key to have 'first', got '%v'", m["test"])
	}

	if err := assignToMap(mv, "test2", []string{"first", "second"}); err == nil {
		//t.Errorf("Expeced an error assigning multiple values to single value. (%v)", m["test2"])
		t.Logf("FIXME: Figure out a way to return an error from a recover()")
	} else if err.Error() != "foo" {
		t.Errorf("Unexpected error: %s (multi-val)", err)
	}
}

type intStruct struct {
	I   int
	I8  int8
	I16 int16
	I32 int32
	I64 int64
}

func TestAssignToInt(t *testing.T) {

	is := &intStruct{8, 8, 8, 8, 8}

	tests := []struct {
		name string
		rv   reflect.Value
		src  string
	}{
		{"int", reflect.ValueOf(&is.I), "64"},
		{"int8", reflect.ValueOf(&is.I8), "8"},
		{"int16", reflect.ValueOf(&is.I16), "16"},
		{"int32", reflect.ValueOf(&is.I32), "32"},
		{"int64", reflect.ValueOf(&is.I64), "64"},
	}

	for _, tt := range tests {
		if err := assignToInt(tt.rv, tt.src); err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%v", reflect.Indirect(tt.rv).Interface()); got != tt.src {
			t.Errorf("Expected %q, got %q", tt.src, got)
		}
	}
}

type AssignmentTestStruct struct {
	FirstName string `form:"first_name"`
	LastName  string
	Year      uint32
	Speed     float64
	Nicknames []string
	IsUseless bool
}

func TestAssignToStruct(t *testing.T) {
	ats := &AssignmentTestStruct{}
	rats := reflect.Indirect(reflect.ValueOf(ats))

	if err := assignToStruct(rats, "first_name", []string{"Matt"}); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if ats.FirstName != "Matt" {
		t.Errorf("Expected ats.FirstName to be Matt, got %q", ats.FirstName)
	}

	if err := assignToStruct(rats, "LastName", []string{"Butcher"}); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if ats.LastName != "Butcher" {
		t.Errorf("Expected ats.LastName to be Butcher, got %q", ats.LastName)
	}

	if err := assignToStruct(rats, "Year", []string{"1999"}); err != nil {
		t.Errorf("Unexpected error assigning Year: %s", err)
	}
	if ats.Year != 1999 {
		t.Errorf("Expected Year=1999, got %d", ats.Year)
	}
	if err := assignToStruct(rats, "Speed", []string{"1.23"}); err != nil {
		t.Errorf("Unexpected error assigning Year: %s", err)
	}
	if ats.Speed != 1.23 {
		t.Errorf("Expected speed=1.23, got %d", ats.Speed)
	}
	if err := assignToStruct(rats, "IsUseless", []string{"true"}); err != nil {
		t.Errorf("Unexpected error assigning Year: %s", err)
	}
	if !ats.IsUseless {
		t.Error("Expected IsUseless to be true")
	}

	if err := assignToStruct(rats, "Nicknames", []string{"John", "Johnny", "Johnboy"}); err != nil {
		t.Errorf("Error converting to []string: %s", err)
	}
}

func fixtureFormData() url.Values {
	v := url.Values{}
	for key, val := range map[string]string{
		"street": "1234 Long St.",
		"city":   "Glenview",
		"state":  "Illinois",
		"zip":    "60626",
	} {
		v.Set(key, val)
	}
	return v
}

type FixtureAddress struct {
	Street string `form:"street"`
	City   string `form:"city"`
	State  string `form:"state"`
	Zip    int    `form:"zip"`
}

func (f *FixtureAddress) FormValidateState(vs []string) *ValidationError {
	if len(vs) == 0 || vs[0] == "" {
		return &ValidationError{"state", "state is required"}
	}
	if vs[0] != "Illinois" {
		return &ValidationError{"state", "unknown state"}
	}
	return nil
}

func (f *FixtureAddress) FormValidateStreet(vs []string) *ValidationError {
	if vs[0] == "FAIL" {
		return &ValidationError{"street", "street cannot be FAIL"}
	}
	return nil
}

func (f *FixtureAddress) FormSetCity(vs []string) error {
	if len(vs) == 0 {
		return errors.New("city is required")
	}
	f.City = strings.ToLower(vs[0])
	fmt.Printf("set city to %s", f.City)
	return nil
}

func TestUnmarshal(t *testing.T) {
	v := fixtureFormData()

	addr := &FixtureAddress{}
	if err := Unmarshal(v, addr); err != nil {
		t.Fatal(err)
	}

	if addr.Street != "1234 Long St." {
		t.Errorf("Unexpected address: %s", addr.Street)
	}

	if addr.City != "glenview" {
		t.Errorf("Unexpected city: %q", addr.State)
	}

	if addr.Zip != 60626 {
		t.Errorf("Unexpected ZIP: %d", addr.Zip)
	}
}

func TestUnmarshalValidator(t *testing.T) {

	v := fixtureFormData()
	v.Set("state", "")
	v.Set("street", "FAIL")

	addr := &FixtureAddress{}
	var _ Validator = addr.FormValidateState
	var _ Validator = addr.FormValidateStreet
	err := Unmarshal(v, addr)
	if err == nil {
		t.Fatalf("Expected validation failure.")
	}

	cve, ok := err.(CompoundValidationError)
	if !ok {
		t.Fatalf("Expected validation error, got %T", err)
	}

	if cve.Len() != 2 {
		t.Errorf("Expected 2 validation errors, got %d", cve.Len())
	}
}

func TestUnmarshalSetter(t *testing.T) {
	v := fixtureFormData()
	v["city"] = []string{}

	addr := &FixtureAddress{}
	var _ Setter = addr.FormSetCity
	if err := Unmarshal(v, addr); err == nil {
		t.Fatalf("Expected fatal validator failure.")
	} else if _, ok := err.(CompoundValidationError); ok {
		t.Fatalf("got validation error instead of fatal error: %s", err)
	}
}
