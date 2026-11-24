// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import (
	"fmt"
	"strings"
)

// PluralI18next names the i18next JSON v4 plural layout: one key per CLDR category, such as
// items_one and items_other.
const PluralI18next = "i18next"

// pluralOrder is the CLDR order of the cardinal plural categories.
var pluralOrder = []string{"zero", "one", "two", "few", "many", "other"}

// pluralRules lists the cardinal plural categories of each language, as CLDR 46 defines them
// for integers and decimals. A language that is not listed gets no categories added.
var pluralRules = func() map[string][]string {
	groups := map[string][]string{
		"other": {"bo", "dz", "id", "ig", "ii", "ja", "jbo", "jv", "kde", "kea", "km", "ko", "lo", "ms", "my",
			"osa", "sah", "ses", "sg", "su", "th", "to", "vi", "wo", "yo", "yue", "zh"},
		"one other": {"af", "am", "as", "az", "bal", "bg", "bn", "ce", "chr", "ckb", "da", "de", "dv", "ee", "el",
			"en", "eo", "et", "eu", "fa", "fi", "fil", "fo", "fy", "gl", "gsw", "gu", "ha", "hi", "hu", "hy",
			"ia", "is", "ka", "kk", "kl", "kn", "ks", "ku", "ky", "lb", "lg", "ln", "mg", "mk", "ml", "mn",
			"mr", "nb", "nd", "ne", "nl", "nn", "no", "nr", "ny", "om", "or", "os", "pa", "ps", "rm", "rw",
			"sd", "si", "sn", "so", "sq", "ss", "st", "sv", "sw", "ta", "te", "tg", "ti", "tk", "tl", "tn",
			"tr", "ts", "ug", "ur", "uz", "ve", "vo", "xh", "yi", "zu"},
		"one many other":              {"ca", "es", "fr", "it", "pt", "vec"},
		"one few other":               {"bs", "hr", "mo", "ro", "sh", "sr"},
		"one few many other":          {"be", "cs", "lt", "pl", "ru", "sk", "uk"},
		"one two other":               {"he", "iu", "se", "sma", "smi", "smj", "smn", "sms"},
		"one two few other":           {"dsb", "gd", "hsb", "sl"},
		"one two few many other":      {"br", "ga", "gv", "mt"},
		"zero one other":              {"ksh", "lag", "lv", "prg"},
		"zero one two few many other": {"ar", "ars", "cy"},
	}

	rules := map[string][]string{}
	for categories, languages := range groups {
		for _, language := range languages {
			rules[language] = strings.Fields(categories)
		}
	}

	return rules
}()

// PluralCategories gives the cardinal plural categories of a locale such as fr, pt-BR or
// zh_Hant, by its language subtag, in CLDR order. It is nil for a language it does not know.
func PluralCategories(locale string) []string {
	language := strings.ToLower(locale)

	if i := strings.IndexAny(language, "-_"); i >= 0 {
		language = language[:i]
	}

	return pluralRules[language]
}

// checkPlurals rejects an unknown plural layout.
func checkPlurals(layout string) error {
	if layout != "" && layout != PluralI18next {
		return fmt.Errorf("plurals must be %q or empty, not %q", PluralI18next, layout)
	}

	return nil
}

// pluralSuffix splits key into the base of an i18next plural group and its category.
// Ordinal keys (items_ordinal_one) are left alone: their categories follow other rules.
func pluralSuffix(key string) (base, category string, ok bool) {
	for _, category := range pluralOrder {
		base, found := strings.CutSuffix(key, "_"+category)

		if found && base != "" && !strings.HasSuffix(base, "_ordinal") {
			return base, category, true
		}
	}

	return "", "", false
}

// expandPlurals copies document, adding to each plural group the categories that the target
// language has and the source lacks. A group is the string keys base_<category> of one object
// that include base_other. An added key reads the base_other source text, so it is translated
// from it, and goes right after the group's last key. Source categories the target lacks stay.
func expandPlurals(document any, target string) any {
	categories := PluralCategories(target)
	if categories == nil {
		return document
	}

	var expand func(any) any

	expand = func(v any) any {
		switch x := v.(type) {
		case *Object:
			out := &Object{Values: make(map[string]any, len(x.Keys))}
			added := map[string][]string{}

			for _, key := range x.Keys {
				base, category, ok := pluralSuffix(key)

				if !ok || category != "other" {
					continue
				}

				if _, isText := x.Values[key].(string); !isText {
					continue
				}

				for _, wanted := range categories {
					name := base + "_" + wanted

					if _, exists := x.Values[name]; !exists {
						added[base] = append(added[base], name)
					}
				}
			}

			last := map[string]int{}

			for i, key := range x.Keys {
				if base, _, ok := pluralSuffix(key); ok && added[base] != nil {
					last[base] = i
				}
			}

			for i, key := range x.Keys {
				out.Keys = append(out.Keys, key)
				out.Values[key] = expand(x.Values[key])
				base, _, ok := pluralSuffix(key)

				if !ok || added[base] == nil || last[base] != i {
					continue
				}

				for _, name := range added[base] {
					out.Keys = append(out.Keys, name)
					out.Values[name] = x.Values[base+"_other"]
				}
			}

			return out

		case []any:
			out := make([]any, len(x))

			for i, item := range x {
				out[i] = expand(item)
			}

			return out

		default:
			return v
		}
	}

	return expand(document)
}
