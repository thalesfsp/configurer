package provider

import (
	"testing"
)

func TestExportToStruct(t *testing.T) {
	type response struct {
		T1 string  `json:"T1"`
		T2 bool    `json:"T2"`
		T3 bool    `json:"T3"`
		T4 string  `json:"T4"`
		T5 float64 `json:"T5"`
		T6 int     `json:"T6"`
	}

	t.Setenv("T1", "text")
	t.Setenv("T2", "true")
	t.Setenv("T3", "false")
	t.Setenv("T4", "HO=HO=HO")
	t.Setenv("T5", "0.64")
	t.Setenv("T6", "12345")

	prov := &Provider{}

	var r response
	if err := prov.ExportToStruct(&r); err != nil {
		t.Fatal(err)
	}

	if r.T1 != "text" {
		t.Fatalf("expected T1 to be 'text', got '%s'", r.T1)
	}
	if r.T2 != true {
		t.Fatal("expected T2 to be true")
	}
	if r.T3 != false {
		t.Fatal("expected T3 to be false")
	}
	if r.T4 != "HO=HO=HO" {
		t.Fatal("expected T4 to be 'HO=HO=HO'")
	}
	if r.T5 != 0.64 {
		t.Fatal("expected T5 to be 0.64")
	}
	if r.T6 != 12345 {
		t.Fatal("expected T6 to be 12345")
	}
}
