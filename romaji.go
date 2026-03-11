package main

// romajiToHiragana converts a romaji string to hiragana, returning
// the converted string and any trailing unconverted romaji buffer.
// For example: "kannji" → ("かんじ", "")
//              "kan"    → ("か", "n")
//              "ky"     → ("", "ky")
func romajiToHiragana(input string) (converted string, pending string) {
	var result []rune
	buf := ""

	for i := 0; i < len(input); i++ {
		ch := input[i]
		buf += string(ch)

		// Double consonant → っ + keep second consonant
		if len(buf) == 2 && buf[0] == buf[1] && isConsonant(buf[0]) && buf[0] != 'n' {
			result = append(result, 'っ')
			buf = string(buf[1])
			continue
		}

		// Try to match the buffer against romaji table (longest match first)
		if kana, ok := romajiMap[buf]; ok {
			result = append(result, []rune(kana)...)
			buf = ""
			continue
		}

		// Special case: "n" followed by a consonant (not y) or end-of-useful-sequence
		if len(buf) >= 2 && buf[0] == 'n' && buf[1] != 'y' && buf[1] != 'a' &&
			buf[1] != 'i' && buf[1] != 'u' && buf[1] != 'e' && buf[1] != 'o' {
			result = append(result, 'ん')
			buf = string(buf[1])
			// Re-check single char
			if kana, ok := romajiMap[buf]; ok {
				result = append(result, []rune(kana)...)
				buf = ""
			}
			continue
		}

		// Check if buffer could still be a prefix of a valid romaji sequence
		if !isPrefix(buf) {
			// Not a valid prefix — output the first byte as-is and retry rest
			result = append(result, rune(buf[0]))
			remaining := buf[1:]
			buf = ""
			// Re-feed remaining characters
			for _, rb := range []byte(remaining) {
				sub, subPending := romajiToHiragana(string(rb))
				result = append(result, []rune(sub)...)
				buf = subPending
			}
		}
	}

	return string(result), buf
}

func isConsonant(b byte) bool {
	switch b {
	case 'k', 's', 't', 'n', 'h', 'm', 'r', 'w', 'g', 'z', 'd', 'b', 'p', 'f', 'j', 'c':
		return true
	}
	return false
}

func isPrefix(s string) bool {
	for key := range romajiMap {
		if len(key) > len(s) && key[:len(s)] == s {
			return true
		}
	}
	// "n" alone is a valid prefix (could become ん or na/ni/etc)
	if s == "n" {
		return true
	}
	return false
}

var romajiMap = map[string]string{
	// Vowels
	"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お",

	// K-row
	"ka": "か", "ki": "き", "ku": "く", "ke": "け", "ko": "こ",
	"kya": "きゃ", "kyu": "きゅ", "kyo": "きょ",

	// S-row
	"sa": "さ", "si": "し", "shi": "し", "su": "す", "se": "せ", "so": "そ",
	"sha": "しゃ", "shu": "しゅ", "sho": "しょ",
	"sya": "しゃ", "syu": "しゅ", "syo": "しょ",

	// T-row
	"ta": "た", "ti": "ち", "chi": "ち", "tu": "つ", "tsu": "つ", "te": "て", "to": "と",
	"cha": "ちゃ", "chu": "ちゅ", "cho": "ちょ",
	"tya": "ちゃ", "tyu": "ちゅ", "tyo": "ちょ",

	// N-row
	"na": "な", "ni": "に", "nu": "ぬ", "ne": "ね", "no": "の",
	"nya": "にゃ", "nyu": "にゅ", "nyo": "にょ",
	"nn": "ん",

	// H-row
	"ha": "は", "hi": "ひ", "hu": "ふ", "fu": "ふ", "he": "へ", "ho": "ほ",
	"hya": "ひゃ", "hyu": "ひゅ", "hyo": "ひょ",

	// M-row
	"ma": "ま", "mi": "み", "mu": "む", "me": "め", "mo": "も",
	"mya": "みゃ", "myu": "みゅ", "myo": "みょ",

	// Y-row
	"ya": "や", "yu": "ゆ", "yo": "よ",

	// R-row
	"ra": "ら", "ri": "り", "ru": "る", "re": "れ", "ro": "ろ",
	"rya": "りゃ", "ryu": "りゅ", "ryo": "りょ",

	// W-row
	"wa": "わ", "wi": "ゐ", "we": "ゑ", "wo": "を",

	// G-row (dakuten)
	"ga": "が", "gi": "ぎ", "gu": "ぐ", "ge": "げ", "go": "ご",
	"gya": "ぎゃ", "gyu": "ぎゅ", "gyo": "ぎょ",

	// Z-row
	"za": "ざ", "zi": "じ", "ji": "じ", "zu": "ず", "ze": "ぜ", "zo": "ぞ",
	"ja": "じゃ", "ju": "じゅ", "jo": "じょ",
	"jya": "じゃ", "jyu": "じゅ", "jyo": "じょ",

	// D-row
	"da": "だ", "di": "ぢ", "du": "づ", "de": "で", "do": "ど",
	"dya": "ぢゃ", "dyu": "ぢゅ", "dyo": "ぢょ",

	// B-row
	"ba": "ば", "bi": "び", "bu": "ぶ", "be": "べ", "bo": "ぼ",
	"bya": "びゃ", "byu": "びゅ", "byo": "びょ",

	// P-row (handakuten)
	"pa": "ぱ", "pi": "ぴ", "pu": "ぷ", "pe": "ぺ", "po": "ぽ",
	"pya": "ぴゃ", "pyu": "ぴゅ", "pyo": "ぴょ",

	// Small kana
	"xa": "ぁ", "xi": "ぃ", "xu": "ぅ", "xe": "ぇ", "xo": "ぉ",
	"xya": "ゃ", "xyu": "ゅ", "xyo": "ょ",
	"xtu": "っ", "xtsu": "っ",

	// Punctuation
	"-": "ー",
}
