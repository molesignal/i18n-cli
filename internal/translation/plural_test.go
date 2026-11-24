// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestTargetsKeepSourceKeyOrder verifies that a translated file lists its keys as the source does.
func TestTargetsKeepSourceKeyOrder(t *testing.T) {
	dir := root(t)
	put(t, filepath.Join(dir, "en", "app.json"), `{"zeta":"Z","alpha":{"second":"2","first":"1"},"mid":"M"}`)

	if err := Run(context.Background(), Options{Dir: dir, Source: "en", Targets: []string{"de"}, BatchSize: 10}, &fake{}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "de", "app.json"))
	if err != nil {
		t.Fatal(err)
	}

	expected := "{\n  \"zeta\": \"de:Z\",\n  \"alpha\": {\n    \"second\": \"de:2\",\n    \"first\": \"de:1\"\n  },\n  \"mid\": \"de:M\"\n}\n"
	if string(data) != expected {
		t.Fatalf("keys out of source order:\n%s", data)
	}
}

// TestPluralCategories verifies the CLDR lookup by language subtag.
func TestPluralCategories(t *testing.T) {
	cases := map[string][]string{
		"fr":      {"one", "many", "other"},
		"pt-BR":   {"one", "many", "other"},
		"ru":      {"one", "few", "many", "other"},
		"ar":      {"zero", "one", "two", "few", "many", "other"},
		"zh_Hant": {"other"},
		"ja-jp":   {"other"},
		"EN-us":   {"one", "other"},
		"xx":      nil,
	}

	for locale, expected := range cases {
		if got := PluralCategories(locale); !reflect.DeepEqual(got, expected) {
			t.Errorf("%s: got %v, want %v", locale, got, expected)
		}
	}
}

// TestPluralFormsFollowTheTargetLanguage verifies that each target gets its own plural categories,
// translated from the source's _other text and placed after the group, and that source
// categories the target lacks are kept.
func TestPluralFormsFollowTheTargetLanguage(t *testing.T) {
	dir := root(t)
	put(t, filepath.Join(dir, "en", "app.json"), `{"items_one":"{{count}} item","items_other":"{{count}} items","title":"Files","rank_ordinal_one":"{{count}}st","rank_ordinal_other":"{{count}}th"}`)
	o := Options{Dir: dir, Source: "en", Targets: []string{"fr", "ru", "ja"}, BatchSize: 20, Plurals: PluralI18next}
	f := &fake{}

	if err := Run(context.Background(), o, f); err != nil {
		t.Fatal(err)
	}

	read := func(locale string) *Object {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, locale, "app.json"))
		if err != nil {
			t.Fatal(err)
		}

		doc, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}

		return doc.(*Object)
	}

	fr := read("fr")
	if want := []string{"items_one", "items_other", "items_many", "title", "rank_ordinal_one", "rank_ordinal_other"}; !reflect.DeepEqual(fr.Keys, want) {
		t.Fatalf("fr keys %v, want %v", fr.Keys, want)
	}

	if fr.Values["items_many"] != "fr:{{count}} items" {
		t.Fatalf("fr many form %q", fr.Values["items_many"])
	}

	ru := read("ru")
	if want := []string{"items_one", "items_other", "items_few", "items_many", "title", "rank_ordinal_one", "rank_ordinal_other"}; !reflect.DeepEqual(ru.Keys, want) {
		t.Fatalf("ru keys %v, want %v", ru.Keys, want)
	}

	ja := read("ja")
	if want := []string{"items_one", "items_other", "title", "rank_ordinal_one", "rank_ordinal_other"}; !reflect.DeepEqual(ja.Keys, want) {
		t.Fatalf("ja keys %v, want %v", ja.Keys, want)
	}

	// 5 source strings in each language, plus fr many and ru few and many.
	if f.entries != 5*3+3 {
		t.Fatalf("translated %d strings", f.entries)
	}

	// A second run keeps the added forms instead of deleting them as target-only keys.
	if err := Run(context.Background(), o, f); err != nil {
		t.Fatal(err)
	}

	if f.entries != 5*3+3 || read("fr").Values["items_many"] != "fr:{{count}} items" {
		t.Fatalf("second run retranslated or dropped plural forms: %d", f.entries)
	}

	// A changed _other text is translated again in every form read from it.
	put(t, filepath.Join(dir, "en", "app.json"), `{"items_one":"{{count}} item","items_other":"{{count}} files","title":"Files","rank_ordinal_one":"{{count}}st","rank_ordinal_other":"{{count}}th"}`)

	if err := Run(context.Background(), o, f); err != nil {
		t.Fatal(err)
	}

	if f.entries != 5*3+3+6 || read("ru").Values["items_few"] != "ru:{{count}} files" {
		t.Fatalf("changed _other was not retranslated in all forms: %d", f.entries)
	}
}

// TestPluralsOffLeavesKeysAlone verifies that without a plural layout no key is added and
// target-only keys are removed as before.
func TestPluralsOffLeavesKeysAlone(t *testing.T) {
	dir := root(t)
	put(t, filepath.Join(dir, "en", "app.json"), `{"items_one":"{{count}} item","items_other":"{{count}} items"}`)

	if err := Run(context.Background(), Options{Dir: dir, Source: "en", Targets: []string{"fr"}, BatchSize: 20}, &fake{}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "fr", "app.json"))
	if err != nil {
		t.Fatal(err)
	}

	doc, _ := Decode(data)
	if keys := doc.(*Object).Keys; !reflect.DeepEqual(keys, []string{"items_one", "items_other"}) {
		t.Fatalf("keys %v", keys)
	}

	if err := Run(context.Background(), Options{Dir: dir, Source: "en", Targets: []string{"fr"}, BatchSize: 20, Plurals: "icu"}, &fake{}); err == nil {
		t.Fatal("an unknown plural layout was accepted")
	}
}
