package util

import (
	"strings"
	"testing"
)

func TestExportToStruct(t *testing.T) {
	t.Setenv("TestExportToStruct_T1", "text1")
	t.Setenv("TestExportToStruct_T2", "false")
	t.Setenv("TestExportToStruct_T3", "1.13")
	t.Setenv("TestExportToStruct_T4", "123")

	type TestData4 struct {
		T8 int `json:"T8" default:"1"`
	}

	type TestData3 struct {
		T7 float64 `json:"T7" default:"0.64"`

		TestData4 TestData4
	}

	type TestData2 struct {
		T5 string `json:"T5" default:"text2"`
		T6 bool   `json:"T6" default:"true"`

		TestData3 *TestData3
	}

	type TestData1 struct {
		T1 string  `json:"T1" default:"text1" env:"TestExportToStruct_T1"`
		T2 bool    `json:"T2" default:"true" env:"TestExportToStruct_T2"`
		T3 float64 `json:"T3" default:"0.64" env:"TestExportToStruct_T3"`
		T4 int     `json:"T4" default:"1" env:"TestExportToStruct_T4"`

		*TestData2
	}

	r := TestData1{
		T1: "text",
		T2: false,
		T3: 3.4,
		T4: 9,

		TestData2: &TestData2{
			T5: "text",
			T6: false,

			TestData3: &TestData3{
				T7: 0,

				TestData4: TestData4{
					T8: 0,
				},
			},
		},
	}
	if err := Dump(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1 != "text1" {
		t.Fatalf("expected T1 to be 'text1', got '%s'", r.T1)
	}
	if r.T2 != false {
		t.Fatal("expected T2 to be true")
	}
	if r.T3 != 1.13 {
		t.Fatal("expected T3 to be 1.13")
	}
	if r.T4 != 123 {
		t.Fatal("expected T4 to be 123")
	}

	if r.T5 != "text" {
		t.Fatalf("expected T5 to be 'text', got '%s'", r.T5)
	}
	if r.T6 != true {
		t.Fatal("expected T6 to be true")
	}
	if r.TestData3.T7 != 0.64 {
		t.Fatal("expected T7 to be 0.64")
	}
	if r.TestData3.TestData4.T8 != 1 {
		t.Fatal("expected T8 to be 1")
	}
}

func TestExportToStruct_validation(t *testing.T) {
	t.Setenv("TestExportToStruct_validation_T1", "text1")
	t.Setenv("TestExportToStruct_validation_T2", "false")
	t.Setenv("TestExportToStruct_validation_T3", "1.13")
	t.Setenv("TestExportToStruct_validation_T4", "123")

	type TestData2 struct {
		T5 string `json:"T5" default:"text2" validate:"gte=10"`
		T6 bool   `json:"T6" default:"true"`
	}

	type TestData1 struct {
		T1 string  `json:"T1" default:"text1" env:"TestExportToStruct_validation_T1"`
		T2 bool    `json:"T2" default:"true" env:"TestExportToStruct_validation_T2"`
		T3 float64 `json:"T3" default:"0.64" env:"TestExportToStruct_validation_T3" validate:"gte=2"`
		T4 int     `json:"T4" default:"1" env:"TestExportToStruct_validation_T4"`

		*TestData2
	}

	r := TestData1{
		T1: "text",
		T2: false,
		T3: 3.4,
		T4: 9,

		TestData2: &TestData2{
			T5: "text",
			T6: false,
		},
	}

	err := Dump(&r)
	if err == nil {
		t.Fatal(err)
	}

	if strings.Contains(err.Error(), "TestData1.TestData2.T5") == false {
		t.Fatal(err)
	}

	if strings.Contains(err.Error(), "TestData1.T3") == false {
		t.Fatal(err)
	}
}

func TestSetDefaults(t *testing.T) {
	type TestData struct {
		T1 string `json:"T1" default:"text"`
		T2 string `json:"T2" default:""`
		T3 string `json:"T3"`
		T4 string `json:"T4" default:"text4"`

		T5 bool `json:"T5" default:"true"`
		T6 bool `json:"T6" default:"false"`
		T7 bool `json:"T7" default:""`
		T8 bool `json:"T8"`

		T9  float64 `json:"T9" default:"0.64"`
		T10 float64 `json:"T10" default:""`
		T11 float64 `json:"T11"`
		T12 float64 `json:"T12" default:"0.33"`

		T13 int `json:"T13" default:"12345"`
		T14 int `json:"T14" default:""`
		T15 int `json:"T15"`
		T16 int `json:"T16" default:"99999"`
	}

	r := TestData{
		T1: "text1", // Should not replace
		T2: "text2", // Should do nothing as no default value is set.
		T3: "text3", // Should do nothing as no default tag is set.
		T4: "",      // Should replace

		T5: false, // Should replace
		T6: false, // Should not replace, value is set.
		T7: false, // Should do nothing as no default value is set.
		T8: false, // Should do nothing as no default tag is set.

		T9:  0.65, // Should not replace
		T10: 0.66, // Should do nothing as no default value is set.
		T11: 0.67, // Should do nothing as no default tag is set.
		T12: 0,    // Should replace

		T13: 12346, // Should not replace
		T14: 12347, // Should do nothing as no default value is set.
		T15: 12348, // Should do nothing as no default tag is set.
		T16: 0,     // Should not replace
	}

	if err := setDefault(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1 != "text1" {
		t.Fatalf("expected T1 to be 'text1', got '%s'", r.T1)
	}
	if r.T2 != "text2" {
		t.Fatalf("expected T2 to be 'text2', got '%s'", r.T2)
	}
	if r.T3 != "text3" {
		t.Fatalf("expected T3 to be 'text3', got '%s'", r.T3)
	}
	if r.T4 != "text4" {
		t.Fatalf("expected T4 to be 'text4', got '%s'", r.T4)
	}
	if r.T5 != true {
		t.Fatal("expected T5 to be true")
	}
	if r.T6 != false {
		t.Fatal("expected T6 to be true")
	}
	if r.T7 != false {
		t.Fatal("expected T7 to be true")
	}
	if r.T8 != false {
		t.Fatal("expected T8 to be true")
	}
	if r.T9 != 0.65 {
		t.Fatal("expected T9 to be 0.65")
	}
	if r.T10 != 0.66 {
		t.Fatal("expected T10 to be 0.66")
	}
	if r.T11 != 0.67 {
		t.Fatal("expected T11 to be 0.67")
	}
	if r.T12 != 0.33 {
		t.Fatal("expected T12 to be 0.33")
	}
	if r.T13 != 12346 {
		t.Fatal("expected T13 to be 12346")
	}
	if r.T14 != 12347 {
		t.Fatal("expected T14 to be 12347")
	}
	if r.T15 != 12348 {
		t.Fatal("expected T15 to be 12348")
	}
	if r.T16 != 99999 {
		t.Fatal("expected T16 to be 99999")
	}
}

func TestSetDefaults_omitempty(t *testing.T) {
	type TestData struct {
		T1 string `json:"T1,omitempty" default:"text"`
		T2 string `json:"T2,omitempty" default:""`
		T3 string `json:"T3,omitempty"`
		T4 string `json:"T4,omitempty" default:"text4"`

		T5 bool `json:"T5,omitempty" default:"true"`
		T6 bool `json:"T6,omitempty" default:"false"`
		T7 bool `json:"T7,omitempty" default:""`
		T8 bool `json:"T8,omitempty"`

		T9  float64 `json:"T9,omitempty" default:"0.64"`
		T10 float64 `json:"T10,omitempty" default:""`
		T11 float64 `json:"T11,omitempty"`
		T12 float64 `json:"T12,omitempty" default:"0.33"`

		T13 int `json:"T13,omitempty" default:"12345"`
		T14 int `json:"T14,omitempty" default:""`
		T15 int `json:"T15,omitempty"`
		T16 int `json:"T16,omitempty" default:"99999"`
	}

	r := TestData{
		T1: "text1", // Should not replace
		T2: "text2", // Should do nothing as no default value is set.
		T3: "",      // Should do nothing as no default tag is set.
		T4: "",      // Should replace

		T5: false, // Should replace
		T6: false, // Should not replace, value is set.
		T7: false, // Should do nothing as no default value is set.
		T8: false, // Should do nothing as no default tag is set.

		T9:  0.65, // Should not replace
		T10: 0.66, // Should do nothing as no default value is set.
		T11: 0,    // Should do nothing as no default tag is set.
		T12: 0,    // Should replace

		T13: 12346, // Should not replace
		T14: 12347, // Should do nothing as no default value is set.
		T15: 0,     // Should do nothing as no default tag is set.
		T16: 0,     // Should not replace
	}

	if err := setDefault(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1 != "text1" {
		t.Fatalf("expected T1 to be 'text1', got '%s'", r.T1)
	}
	if r.T2 != "text2" {
		t.Fatalf("expected T2 to be 'text2', got '%s'", r.T2)
	}
	if r.T3 != "" {
		t.Fatalf("expected T3 to be '', got '%s'", r.T3)
	}
	if r.T4 != "text4" {
		t.Fatalf("expected T4 to be 'text4', got '%s'", r.T4)
	}
	if r.T5 != true {
		t.Fatal("expected T5 to be true")
	}
	if r.T6 != false {
		t.Fatal("expected T6 to be true")
	}
	if r.T7 != false {
		t.Fatal("expected T7 to be true")
	}
	if r.T8 != false {
		t.Fatal("expected T8 to be true")
	}
	if r.T9 != 0.65 {
		t.Fatal("expected T9 to be 0.65")
	}
	if r.T10 != 0.66 {
		t.Fatal("expected T10 to be 0.66")
	}
	if r.T11 != 0 {
		t.Fatal("expected T11 to be 0")
	}
	if r.T12 != 0.33 {
		t.Fatal("expected T12 to be 0.33")
	}
	if r.T13 != 12346 {
		t.Fatal("expected T13 to be 12346")
	}
	if r.T14 != 12347 {
		t.Fatal("expected T14 to be 12347")
	}
	if r.T15 != 0 {
		t.Fatal("expected T15 to be 0")
	}
	if r.T16 != 99999 {
		t.Fatal("expected T16 to be 99999")
	}
}

func TestSetEnv(t *testing.T) {
	t.Setenv("TestSetEnv_T1", "text1")
	t.Setenv("TestSetEnv_T2", "")
	t.Setenv("TestSetEnv_T3", "true")
	t.Setenv("TestSetEnv_T4", "false")
	t.Setenv("TestSetEnv_T5", "0.65")
	t.Setenv("TestSetEnv_T6", "0")
	t.Setenv("TestSetEnv_T7", "123")
	t.Setenv("TestSetEnv_T8", "")

	type TestData struct {
		T1 string `env:"TestSetEnv_T1"`
		T2 string `env:"TestSetEnv_T2"`

		T3 bool `env:"TestSetEnv_T3"`
		T4 bool `env:"TestSetEnv_T4"`

		T5 float64 `env:"TestSetEnv_T5"`
		T6 float64 `env:"TestSetEnv_T6"`

		T7 int `env:"TestSetEnv_T7"`
		T8 int `env:"TestSetEnv_T8"`
	}

	var r TestData
	if err := setEnv(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1 != "text1" {
		t.Fatalf("expected T1 to be 'text1', got '%s'", r.T1)
	}
	if r.T2 != "" {
		t.Fatalf("expected T2 to be '', got '%s'", r.T2)
	}
	if r.T3 != true {
		t.Fatal("expected T3 to be true")
	}
	if r.T4 != false {
		t.Fatal("expected T4 to be false")
	}
	if r.T5 != 0.65 {
		t.Fatal("expected T5 to be 0.65")
	}
	if r.T6 != 0 {
		t.Fatal("expected T6 to be 0")
	}
	if r.T7 != 123 {
		t.Fatal("expected T7 to be 123")
	}
	if r.T8 != 0 {
		t.Fatal("expected T8 to be 0")
	}
}
