package router

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// EstimateTokens follows router-core 0.3's CJK-aware, model-agnostic estimate.
func EstimateTokens(text string) int {
	cjk, other := 0, 0
	for _, r := range text {
		if isCJK(r) {
			cjk++
		} else {
			other += utf16.RuneLen(r)
		}
	}
	return cjk + (other+3)/4
}

func isCJK(r rune) bool {
	return r >= 0x3000 && r <= 0x30ff || r >= 0x3400 && r <= 0x4dbf || r >= 0x4e00 && r <= 0x9fff || r >= 0xac00 && r <= 0xd7af || r >= 0xf900 && r <= 0xfaff || r >= 0xff00 && r <= 0xffef
}

func sampleForScan(text string, limit int) string {
	if limit < 1 {
		limit = 1
	}
	if limit > 8000 {
		limit = 8000
	}
	r := []rune(text)
	if len(r) <= limit {
		return text
	}
	left := (limit + 1) / 2
	return string(r[:left]) + "\n" + string(r[len(r)-(limit-left):])
}

func segmentedWord(r rune) bool {
	return unicode.Is(unicode.Latin, r) || unicode.Is(unicode.Cyrillic, r) || unicode.Is(unicode.Greek, r) || unicode.IsNumber(r)
}

// matchesKeyword preserves stems and inflections without matching unrelated words.
func matchesKeyword(text, keyword string) bool {
	kw := strings.ToLower(keyword)
	if kw == "" {
		return true
	}
	for _, r := range kw {
		if isCJK(r) || r >= 0x600 && r <= 0x6ff {
			return strings.Contains(text, kw)
		}
	}
	last := kw[strings.LastIndex(kw, " ")+1:]
	ascii := true
	for _, r := range kw {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '\'' || r == ' ' || r == '-') {
			ascii = false
		}
	}
	forms := []string{kw}
	end, _ := utf8.DecodeLastRuneInString(kw)
	bounded := ascii && len(last) <= 6 && (unicode.IsLetter(end) || unicode.IsNumber(end))
	if bounded && !unicode.IsDigit(end) {
		suffixes := []string{"s", "ed", "ing"}
		if len(last) <= 3 {
			if strings.HasSuffix(kw, "x") || strings.HasSuffix(kw, "s") || strings.HasSuffix(kw, "z") || strings.HasSuffix(kw, "ch") || strings.HasSuffix(kw, "sh") {
				suffixes = append(suffixes, "es")
			}
		} else {
			suffixes = []string{"s", "es", "ed", "d", "ing", "er", "ers", "ly", "ment", "ments", "ation", "ations", "e", "en", "n", "a", "as", "o", "os", "em", "et", "ung", "ungen"}
			if end == 'e' {
				for _, suffix := range []string{"ing", "ed", "es", "er", "ers", "ation", "ations"} {
					forms = append(forms, kw[:len(kw)-1]+suffix)
				}
			} else if end == 'y' {
				forms = append(forms, kw[:len(kw)-1]+"ies", kw[:len(kw)-1]+"ied")
			} else if len(kw) >= 2 && strings.ContainsRune("aeiou", rune(kw[len(kw)-2])) && strings.ContainsRune("bdgmnprt", end) {
				for _, suffix := range []string{"ing", "ed", "er", "ers"} {
					forms = append(forms, kw+string(end)+suffix)
				}
			}
		}
		for _, suffix := range suffixes {
			forms = append(forms, kw+suffix)
		}
	}
	first, _ := utf8.DecodeRuneInString(kw)
	for _, form := range forms {
		for from := 0; from <= len(text); {
			pos := strings.Index(text[from:], form)
			if pos < 0 {
				break
			}
			pos += from
			before, _ := utf8.DecodeLastRuneInString(text[:pos])
			after, _ := utf8.DecodeRuneInString(text[pos+len(form):])
			if (!(unicode.IsLetter(first) || unicode.IsNumber(first)) || pos == 0 || !segmentedWord(before)) && (!bounded || !segmentedWord(after)) {
				return true
			}
			from = pos + 1
		}
	}
	return false
}
