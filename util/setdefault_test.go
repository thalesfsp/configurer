package util

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSetDefaults(t *testing.T) {
	t.Run("Test string defaults", testStringDefaults)

	t.Run("Test bool defaults", testBoolDefaults)

	t.Run("Test float64 defaults", testFloat64Defaults)

	t.Run("Test int defaults", testIntDefaults)

	t.Run("Test slice defaults", testSliceDefaults)

	t.Run("Test map defaults", testMapDefaults)

	t.Run("Test time.Duration defaults", testDurationDefaults)

	t.Run("Test time.Time defaults", testTimeDefaults)
}

func testStringDefaults(t *testing.T) {
	type TestData struct {
		T1 string `json:"T1" default:"text"`
		T2 string `json:"T2" default:""`
		T3 string `json:"T3"`
		T4 string `json:"T4" default:"text4"`
	}

	r := TestData{
		T1: "text1", // Should not replace
		T2: "text2", // Should do nothing as no default value is set.
		T3: "text3", // Should do nothing as no default tag is set.
		T4: "",      // Should replace
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
	if r.T3 != "text3" {
		t.Fatalf("expected T3 to be 'text3', got '%s'", r.T3)
	}
	if r.T4 != "text4" {
		t.Fatalf("expected T4 to be 'text4', got '%s'", r.T4)
	}
}

func testBoolDefaults(t *testing.T) {
	type TestData struct {
		T1 bool `json:"T1" default:"true"`
		T2 bool `json:"T2" default:"false"`
		T3 bool `json:"T3" default:""`
		T4 bool `json:"T4"`
	}

	r := TestData{
		T1: false, // Should replace
		T2: false, // Should not replace, value is set.
		T3: false, // Should do nothing as no default value is set.
		T4: false, // Should do nothing as no default tag is set.
	}

	if err := SetDefault(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1 != true {
		t.Fatal("expected T1 to be true")
	}
	if r.T2 != false {
		t.Fatal("expected T2 to be false")
	}
	if r.T3 != false {
		t.Fatal("expected T3 to be false")
	}
	if r.T4 != false {
		t.Fatal("expected T4 to be false")
	}
}

func testFloat64Defaults(t *testing.T) {
	type TestData struct {
		T1 float64 `json:"T1" default:"0.64"`
		T2 float64 `json:"T2" default:""`
		T3 float64 `json:"T3"`
		T4 float64 `json:"T4" default:"0.33"`
	}

	r := TestData{
		T1: 0.65, // Should not replace
		T2: 0.66, // Should do nothing as no default value is set.
		T3: 0.67, // Should do nothing as no default tag is set.
		T4: 0,    // Should replace
	}

	if err := SetDefault(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1 != 0.65 {
		t.Fatal("expected T1 to be 0.65")
	}
	if r.T2 != 0.66 {
		t.Fatal("expected T2 to be 0.66")
	}
	if r.T3 != 0.67 {
		t.Fatal("expected T3 to be 0.67")
	}
	if r.T4 != 0.33 {
		t.Fatal("expected T4 to be 0.33")
	}
}

func testIntDefaults(t *testing.T) {
	type TestData struct {
		T1 int `json:"T1" default:"12345"`
		T2 int `json:"T2" default:""`
		T3 int `json:"T3"`
		T4 int `json:"T4" default:"99999"`
	}

	r := TestData{
		T1: 12346, // Should not replace
		T2: 12347, // Should do nothing as no default value is set.
		T3: 12348, // Should do nothing as no default tag is set.
		T4: 0,     // Should replace
	}

	if err := SetDefault(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1 != 12346 {
		t.Fatal("expected T1 to be 12346")
	}
	if r.T2 != 12347 {
		t.Fatal("expected T2 to be 12347")
	}
	if r.T3 != 12348 {
		t.Fatal("expected T3 to be 12348")
	}
	if r.T4 != 99999 {
		t.Fatal("expected T4 to be 99999")
	}
}

func testSliceDefaults(t *testing.T) {
	type TestData struct {
		T1 []string        `default:"asd,qwe,dfg"`
		T2 []int           `default:"1,2,3"`
		T3 []float64       `default:"0.65,0.66,0.67"`
		T4 []bool          `default:"true,false,true"`
		T5 []time.Duration `default:"1s,2s,3s"`
		T6 []time.Time     `default:"2018-01-01,2019-01-02,2020-01-03"`
		T7 []string        `default:"[]"`
	}

	r := TestData{}

	if err := SetDefault(&r); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(r.T1, []string{"asd", "qwe", "dfg"}) {
		t.Fatal("expected T1 to be ['asd', 'qwe', 'dfg']")
	}
	if !reflect.DeepEqual(r.T2, []int{1, 2, 3}) {
		t.Fatal("expected T2 to be [1, 2, 3]")
	}
	if !reflect.DeepEqual(r.T3, []float64{0.65, 0.66, 0.67}) {
		t.Fatal("expected T3 to be [0.65, 0.66, 0.67]")
	}
	if !reflect.DeepEqual(r.T4, []bool{true, false, true}) {
		t.Fatal("expected T4 to be [true, false, true]")
	}
	if !reflect.DeepEqual(r.T5, []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second}) {
		t.Fatal("expected T4 to be [1s 2s 3s]")
	}
	if r.T6[0].Year() != 2018 || r.T6[1].Year() != 2019 || r.T6[2].Year() != 2020 {
		t.Fatal("expected T6 to be [2018-01-01 2019-01-02 2020-01-03]")
	}
	if r.T7 == nil {
		t.Fatal("expected T7 to be []")
	}
}

func testMapDefaults(t *testing.T) {
	type TestData struct {
		T1  map[string]interface{}   `default:"asd:qwe,dfg:1"`
		T2  map[string]interface{}   `default:"asd:qwe,dfg:text1"`
		T3  map[string]interface{}   `default:"asd:qwe,dfg:true"`
		T4  map[string]interface{}   `default:"asd:qwe,dfg:false"`
		T5  map[string]interface{}   `default:"asd:qwe,dfg:0.65"`
		T6  map[string]interface{}   `default:"asd:qwe,dfg:0"`
		T7  map[string]interface{}   `default:"asd:qwe,dfg:123"`
		T8  map[string]string        `default:"asd:qwe,dfg:iu"`
		T9  map[string]int           `default:"asd:123,dfg:456"`
		T10 map[string]float64       `default:"asd:0.65,dfg:0.66"`
		T11 map[string]bool          `default:"asd:true,dfg:false"`
		T12 map[string]time.Duration `default:"asd:1s,dfg:2s"`
		T13 map[string]time.Time     `default:"asd:2018-01-01,dfg:2019-01-02"`
		T14 map[string]interface{}   `default:"[]"` // Should be empty map
		T15 map[string]interface{}   `default:""`   // Should be empty map
	}

	r := TestData{}

	if err := SetDefault(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1["asd"] != "qwe" || r.T1["dfg"] != 1 {
		t.Fatal("expected T1 to be {'asd': 'qwe', 'dfg': 1}")
	}
	if !reflect.DeepEqual(r.T2, map[string]interface{}{"asd": "qwe", "dfg": "text1"}) {
		t.Fatal("expected T2 to be {'asd': 'qwe', 'dfg': 'text1'}")
	}
	if !reflect.DeepEqual(r.T3, map[string]interface{}{"asd": "qwe", "dfg": true}) {
		t.Fatal("expected T3 to be {'asd': 'qwe', 'dfg': true}")
	}
	if !reflect.DeepEqual(r.T4, map[string]interface{}{"asd": "qwe", "dfg": false}) {
		t.Fatal("expected T4 to be {'asd': 'qwe', 'dfg': false}")
	}
	if !reflect.DeepEqual(r.T5, map[string]interface{}{"asd": "qwe", "dfg": 0.65}) {
		t.Fatal("expected T5 to be {'asd': 'qwe', 'dfg': 0.65}")
	}
	if !reflect.DeepEqual(r.T6, map[string]interface{}{"asd": "qwe", "dfg": 0}) {
		t.Fatal("expected T6 to be {'asd': 'qwe', 'dfg': 0}")
	}
	if !reflect.DeepEqual(r.T7, map[string]interface{}{"asd": "qwe", "dfg": 123}) {
		t.Fatal("expected T7 to be {'asd': 'qwe', 'dfg': 123}")
	}

	assert.Equal(t, map[string]string{"asd": "qwe", "dfg": "iu"}, r.T8)
	assert.Equal(t, map[string]int{"asd": 123, "dfg": 456}, r.T9)
	assert.Equal(t, map[string]float64{"asd": 0.65, "dfg": 0.66}, r.T10)
	assert.Equal(t, map[string]bool{"asd": true, "dfg": false}, r.T11)
	assert.Equal(t, map[string]time.Duration{"asd": time.Second, "dfg": time.Second * 2}, r.T12)
	assert.Equal(t, map[string]time.Time{
		"asd": time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC),
		"dfg": time.Date(2019, 1, 2, 0, 0, 0, 0, time.UTC),
	},
		r.T13,
	)
	assert.Equal(t, map[string]interface{}{}, r.T14)
	assert.Equal(t, map[string]interface{}{}, r.T15)
}

func testDurationDefaults(t *testing.T) {
	type TestData struct {
		T1 time.Duration `default:"33s"`
	}

	r := TestData{}

	if err := SetDefault(&r); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(r.T1, time.Second*33) {
		t.Fatal("expected T1 to be 33s")
	}
}

func testTimeDefaults(t *testing.T) {
	type TestData struct {
		T1 time.Time `default:"2023/03/20"`
		T2 time.Time `default:""`
		T3 time.Time `default:"2023/03/20"` // Should not replace.
	}

	expectedDate := time.Date(2023, 0o3, 20, 0, 0, 0, 0, time.UTC)

	r := TestData{
		T3: expectedDate,
	}

	if err := SetDefault(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1.Year() != 2023 {
		t.Fatal("expected T1 to be 2023")
	}

	if nowYear := time.Now().Year(); r.T2.Year() != nowYear {
		t.Fatalf("expected T2 to be %d", nowYear)
	}

	if r.T3.Year() != 2023 {
		t.Fatal("expected T3 to be 2023")
	}
}
