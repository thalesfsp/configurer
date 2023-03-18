package util

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDump(t *testing.T) {
	t.Setenv("TestDump_T1", "text1")
	t.Setenv("TestDump_T2", "true")
	t.Setenv("TestDump_T3", "1.13")
	t.Setenv("TestDump_T4", "123")

	type TestData4 struct {
		T8 int `json:"T8" default:"1"`
	}

	type TestData3 struct {
		T7 float64 `json:"T7" default:"0.64"`

		// TestData4 TestData4
	}

	type TestData2 struct {
		// T5 string `json:"T5" default:"text2"`
		// T6 bool   `json:"T6" default:"true"`

		TestData3 *TestData3
	}

	type TestData1 struct {
		// T1 string  `json:"T1" default:"text1" env:"TestDump_T1"`
		// T2 bool    `json:"T2" default:"false" env:"TestDump_T2"`
		// T3 float64 `json:"T3" default:"0.64" env:"TestDump_T3"`
		// T4 int     `json:"T4" default:"1" env:"TestDump_T4"`

		*TestData2
	}

	r := TestData1{
		TestData2: &TestData2{
			// T5: "text",
			// T6: false,

			TestData3: &TestData3{
				T7: 0,

				// TestData4: TestData4{
				// 	T8: 0,
				// },
			},
		},
	}
	if err := Dump(&r); err != nil {
		t.Fatal(err)
	}

	// if r.T1 != "text1" {
	// 	t.Fatalf("expected T1 to be 'text1', got '%s'", r.T1)
	// }
	// if r.T2 != true {
	// 	t.Fatal("expected T2 to be true")
	// }
	// if r.T3 != 1.13 {
	// 	t.Fatal("expected T3 to be 1.13 got", r.T3)
	// }
	// if r.T4 != 123 {
	// 	t.Fatal("expected T4 to be 123")
	// }
	// if r.T5 != "text" {
	// 	t.Fatalf("expected T5 to be 'text', got '%s'", r.T5)
	// }
	// if r.T6 != true {
	// 	t.Fatal("expected T6 to be true")
	// }
	if r.TestData3.T7 != 0.64 {
		t.Fatal("expected T7 to be 0.64 got", r.TestData3.T7)
	}
	// if r.TestData3.TestData4.T8 != 1 {
	// 	t.Fatal("expected T8 to be 1")
	// }
}

func TestDump_validation(t *testing.T) {
	t.Setenv("TestDump_validation_T1", "text1")
	t.Setenv("TestDump_validation_T2", "false")
	t.Setenv("TestDump_validation_T3", "1.13")
	t.Setenv("TestDump_validation_T4", "123")

	type TestData2 struct {
		T5 string `json:"T5" default:"text2" validate:"gte=10"`
		T6 bool   `json:"T6" default:"true"`
	}

	type TestData1 struct {
		T1 string  `json:"T1" default:"text1" env:"TestDump_validation_T1"`
		T2 bool    `json:"T2" default:"true" env:"TestDump_validation_T2"`
		T3 float64 `json:"T3" default:"0.64" env:"TestDump_validation_T3" validate:"gte=2"`
		T4 int     `json:"T4" default:"1" env:"TestDump_validation_T4"`

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

	if err := Dump(&r); err == nil {
		t.Fatal(err)
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

	if err := SetDefault(&r); err != nil {
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
	t.Setenv("TestSetEnv_T9", "5m")
	t.Setenv("TestSetEnv_T10", "asd,qwe,dfg")
	t.Setenv("TestSetEnv_T11", "asd:qwe,dfg:1")
	t.Setenv("TestSetEnv_T12", "asd:qwe,dfg:text1")
	t.Setenv("TestSetEnv_T13", "asd:qwe,dfg:true")
	t.Setenv("TestSetEnv_T14", "asd:qwe,dfg:false")
	t.Setenv("TestSetEnv_T15", "asd:qwe,dfg:0.65")
	t.Setenv("TestSetEnv_T16", "asd:qwe,dfg:0")
	t.Setenv("TestSetEnv_T17", "asd:qwe,dfg:123")
	t.Setenv("TestSetEnv_T18", "1,2,3")
	t.Setenv("TestSetEnv_T19", "0.65,0.66,0.67")
	t.Setenv("TestSetEnv_T20", "true,false,true")

	type TestData struct {
		T1 string `env:"TestSetEnv_T1"`
		T2 string `env:"TestSetEnv_T2"`

		T3 bool `env:"TestSetEnv_T3"`
		T4 bool `env:"TestSetEnv_T4"`

		T5 float64 `env:"TestSetEnv_T5"`
		T6 float64 `env:"TestSetEnv_T6"`

		T7 int `env:"TestSetEnv_T7"`
		T8 int `env:"TestSetEnv_T8"`

		T9 time.Duration `env:"TestSetEnv_T9"`

		T10 []string `env:"TestSetEnv_T10"`

		T11 map[string]interface{} `env:"TestSetEnv_T11"`
		T12 map[string]interface{} `env:"TestSetEnv_T12"`
		T13 map[string]interface{} `env:"TestSetEnv_T13"`
		T14 map[string]interface{} `env:"TestSetEnv_T14"`
		T15 map[string]interface{} `env:"TestSetEnv_T15"`
		T16 map[string]interface{} `env:"TestSetEnv_T16"`
		T17 map[string]interface{} `env:"TestSetEnv_T17"`

		T18 []int     `env:"TestSetEnv_T18"`
		T19 []float64 `env:"TestSetEnv_T19"`
		T20 []bool    `env:"TestSetEnv_T20"`
	}

	var r TestData
	if err := SetEnv(&r); err != nil {
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
	if r.T9 != 5*time.Minute {
		t.Fatal("expected T9 to be 5 minutes")
	}
	if !reflect.DeepEqual(r.T10, []string{"asd", "qwe", "dfg"}) {
		t.Fatal("expected T10 to be [asd qwe dfg]", r.T10)
	}
	if !reflect.DeepEqual(r.T11, map[string]interface{}{"asd": "qwe", "dfg": 1}) {
		t.Fatal("expected T11 to be [asd:qwe dfg:1]", r.T11)
	}
	if !reflect.DeepEqual(r.T12, map[string]interface{}{"asd": "qwe", "dfg": "text1"}) {
		t.Fatal("expected T12 to be [asd:qwe dfg:text1]", r.T12)
	}
	if !reflect.DeepEqual(r.T13, map[string]interface{}{"asd": "qwe", "dfg": true}) {
		t.Fatal("expected T13 to be [asd:qwe dfg:true]", r.T13)
	}
	if !reflect.DeepEqual(r.T14, map[string]interface{}{"asd": "qwe", "dfg": false}) {
		t.Fatal("expected T14 to be [asd:qwe dfg:false]", r.T14)
	}
	if !reflect.DeepEqual(r.T15, map[string]interface{}{"asd": "qwe", "dfg": 0.65}) {
		t.Fatal("expected T15 to be [asd:qwe dfg:0.65]", r.T15)
	}
	if !reflect.DeepEqual(r.T16, map[string]interface{}{"asd": "qwe", "dfg": 0}) {
		t.Fatal("expected T16 to be [asd:qwe dfg:0]", r.T16)
	}
	if !reflect.DeepEqual(r.T17, map[string]interface{}{"asd": "qwe", "dfg": 123}) {
		t.Fatal("expected T17 to be [asd:qwe dfg:123]", r.T17)
	}
	if !reflect.DeepEqual(r.T18, []int{1, 2, 3}) {
		t.Fatal("expected T18 to be [1 2 3]", r.T18)
	}
	if !reflect.DeepEqual(r.T19, []float64{0.65, 0.66, 0.67}) {
		t.Fatal("expected T19 to be [0.65 0.66 0.67]", r.T19)
	}
	if !reflect.DeepEqual(r.T20, []bool{true, false, true}) {
		t.Fatal("expected T20 to be [true false true]", r.T20)
	}
}

func TestDumpToEnv(t *testing.T) {
	file, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}

	defer file.Close()

	if err := DumpToEnv(file, map[string]string{
		"K1": "V1",
		"K2": "V2",
	}); err != nil {
		t.Fatal(err)
	}

	// Read and print the contents of the file.
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "K1=V1") {
		t.Fatal("expected file to contain K1=V1")
	}

	if !strings.Contains(string(data), "K2=V2") {
		t.Fatal("expected file to contain K2=V2")
	}
}

func TestSetID(t *testing.T) {
	type TestData struct {
		ID string `id:""`
	}

	testCases := []struct {
		name         string
		input        TestData
		expectedID   string
		expectedLen  int
		expectErrMsg bool
	}{
		{
			name:        "empty id",
			input:       TestData{},
			expectedLen: 36,
		},
		{
			name: "specifying id",
			input: TestData{
				ID: "123123123",
			},
			expectedID:  "123123123",
			expectedLen: 9,
		},
		{
			name: "with ID, not specifying id",
			input: TestData{
				ID: "51515151",
			},
			expectedID:  "51515151",
			expectedLen: 8,
		},
		{
			name: "with ID, specifying id",
			input: TestData{
				ID: "7878787878",
			},
			expectedID:  "7878787878",
			expectedLen: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := SetID(&tc.input)
			if err != nil {
				if !tc.expectErrMsg {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if tc.input.ID == "" {
				t.Fatal("expected ID to be set")
			}

			if tc.expectedID != "" && tc.input.ID != tc.expectedID {
				t.Fatalf("expected ID to be %q, but got %q", tc.expectedID, tc.input.ID)
			}

			if len(tc.input.ID) != tc.expectedLen {
				t.Fatalf("expected ID length to be %d, but got %d", tc.expectedLen, len(tc.input.ID))
			}
		})
	}
}
