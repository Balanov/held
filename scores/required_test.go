package scores

import (
	"strings"
	"testing"
)

func TestLoadFromRequiresKnownScore(t *testing.T) {
	for _, tc := range []struct{ name, record, want string }{
		{"missing", "{\"split\":\"hold\",\"system\":\"a\",\"item\":\"one\"}", "score"},
		{"misspelled", "{\"split\":\"hold\",\"system\":\"a\",\"item\":\"one\",\"scroe\":1}", "scroe"},
		{"unknown", "{\"split\":\"hold\",\"system\":\"a\",\"item\":\"one\",\"score\":1,\"extra\":1}", "extra"},
		{"null", "{\"split\":\"hold\",\"system\":\"a\",\"item\":\"one\",\"score\":null}", "score"},
	} {
		for _, array := range []bool{false, true} {
			input, where := tc.record, "line 1"
			if array {
				input, where = "["+input+"]", "record 1"
			}
			t.Run(tc.name+"/"+where, func(t *testing.T) {
				_, err := LoadFrom(strings.NewReader(input))
				if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), where) {
					t.Fatalf("expected %s and %s, got %v", where, tc.want, err)
				}
			})
		}
	}
	for _, score := range []string{"0", "0.5", "1"} {
		input := "{\"split\":\"hold\",\"system\":\"a\",\"item\":\"one\",\"score\":" + score + "}"
		for _, data := range []string{input, "[" + input + "]"} {
			if _, err := LoadFrom(strings.NewReader(data)); err != nil {
				t.Fatalf("explicit %s: %v", score, err)
			}
		}
	}
}
