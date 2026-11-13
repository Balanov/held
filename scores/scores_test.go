package scores

import (
	"strings"
	"testing"
)

func TestLoadJSONLines(t *testing.T) {
	d, err := LoadFrom(strings.NewReader(
		"{\"split\":\"sel\",\"system\":\"base\",\"item\":\"a\",\"score\":1}\n" +
			"{\"split\":\"sel\",\"system\":\"cand\",\"item\":\"a\",\"score\":0}\n"))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(d.Splits) != 1 {
		t.Fatalf("splits = %d", len(d.Splits))
	}
}

func TestLoadJSONArray(t *testing.T) {
	d, err := LoadFrom(strings.NewReader(
		"[{\"split\":\"sel\",\"system\":\"base\",\"item\":\"a\",\"score\":1}]"))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got := d.Systems("sel"); len(got) != 1 || got[0] != "base" {
		t.Errorf("systems = %v", got)
	}
}

func TestBothFormatsAgree(t *testing.T) {
	line := "{\"split\":\"sel\",\"system\":\"base\",\"item\":\"a\",\"score\":1}"
	array := "[" + line + "]"
	a, err := LoadFrom(strings.NewReader(line))
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadFrom(strings.NewReader(array))
	if err != nil {
		t.Fatal(err)
	}
	da, _, _ := a.Pair("sel", "base", "base")
	db, _, _ := b.Pair("sel", "base", "base")
	if len(da) != len(db) {
		t.Fatalf("pair counts differ: %v %v", da, db)
	}
}

func TestPairKeepsOnlySharedItems(t *testing.T) {
	d, err := LoadFrom(strings.NewReader(
		"{\"split\":\"sel\",\"system\":\"base\",\"item\":\"a\",\"score\":1}\n" +
			"{\"split\":\"sel\",\"system\":\"base\",\"item\":\"b\",\"score\":0}\n" +
			"{\"split\":\"sel\",\"system\":\"cand\",\"item\":\"a\",\"score\":0}\n"))
	if err != nil {
		t.Fatal(err)
	}
	diffs, items, err := d.Pair("sel", "base", "cand")
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || len(items) != 1 {
		t.Fatalf("diffs = %v items = %v", diffs, items)
	}
	if diffs[0] != -1 {
		t.Errorf("diff = %v, want -1", diffs[0])
	}
}

func TestPairIsSorted(t *testing.T) {
	d, _ := LoadFrom(strings.NewReader(
		"{\"split\":\"s\",\"system\":\"b\",\"item\":\"z\",\"score\":1}\n" +
			"{\"split\":\"s\",\"system\":\"b\",\"item\":\"a\",\"score\":0}\n" +
			"{\"split\":\"s\",\"system\":\"c\",\"item\":\"z\",\"score\":0}\n" +
			"{\"split\":\"s\",\"system\":\"c\",\"item\":\"a\",\"score\":1}\n"))
	_, items, err := d.Pair("s", "b", "c")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0] != "a" || items[1] != "z" {
		t.Errorf("items = %v", items)
	}
}

func TestRejectsBadRecords(t *testing.T) {
	cases := []string{
		"{\"split\":\"\",\"system\":\"b\",\"item\":\"a\",\"score\":1}",
		"{\"split\":\"s\",\"system\":\"\",\"item\":\"a\",\"score\":1}",
		"{\"split\":\"s\",\"system\":\"b\",\"item\":\"\",\"score\":1}",
		"{\"split\":\"s\",\"system\":\"b\",\"item\":\"a\",\"score\":2}",
		"{\"split\":\"s\",\"system\":\"b\",\"item\":\"a\",\"score\":-1}",
	}
	for _, c := range cases {
		if _, err := LoadFrom(strings.NewReader(c)); err == nil {
			t.Errorf("expected an error for %s", c)
		}
	}
}

func TestRejectsDuplicate(t *testing.T) {
	rec := "{\"split\":\"s\",\"system\":\"b\",\"item\":\"a\",\"score\":1}\n"
	if _, err := LoadFrom(strings.NewReader(rec + rec)); err == nil {
		t.Error("expected an error for a duplicate record")
	}
}

func TestEmptyInput(t *testing.T) {
	if _, err := LoadFrom(strings.NewReader("   \n")); err == nil {
		t.Error("expected an error for empty input")
	}
}

func TestMissingSplitIsAnError(t *testing.T) {
	d, _ := LoadFrom(strings.NewReader(
		"{\"split\":\"s\",\"system\":\"b\",\"item\":\"a\",\"score\":1}\n"))
	if _, _, err := d.Pair("other", "b", "b"); err == nil {
		t.Error("expected an error for a missing split")
	}
}
