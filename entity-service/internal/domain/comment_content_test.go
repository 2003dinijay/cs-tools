package domain

import "testing"

func TestStripCommentCodeMarkers(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"wrapped", "[code]I'm sorry, happy to help.[/code]", "I'm sorry, happy to help."},
		{"unwrapped", "plain text", "plain text"},
		{"empty", "", ""},
		{"only leading", "[code]no closer", "[code]no closer"},
		{"only trailing", "no opener[/code]", "no opener[/code]"},
		{"surrounding whitespace", "  \n[code]body[/code]\n ", "body"},
		{"interior whitespace kept", "[code]  body\n[/code]", "  body\n"},
		{"interior markers untouched", "[code]a [code]b[/code] c[/code]", "a [code]b[/code] c"},
		{"interior only", "see [code]x[/code] here", "see [code]x[/code] here"},
		{"html untouched", "[code]<b>hi</b>[/code]", "<b>hi</b>"},
		{"empty wrapper", "[code][/code]", ""},
		{"unwrapped keeps whitespace", "  text  ", "  text  "},
		{"overlapping markers", "[code]", "[code]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripCommentCodeMarkers(tt.in); got != tt.want {
				t.Errorf("StripCommentCodeMarkers(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
