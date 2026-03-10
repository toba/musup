package state

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Loser (radio edit)", "loser"},
		{"Away From The Sun", "away from the sun"},
		{"Away from the Sun", "away from the sun"},
		{"Be Like That [Live]", "be like that"},
		{"The Better Life (Deluxe Edition)", "the better life"},
		{"Don't Stop Believin'", "dont stop believin"},
		{"Rock & Roll", "rock roll"},
		{"  Hello   World  ", "hello world"},
		{"", ""},
		{"OK Computer", "ok computer"},
		{"Hail to the Thief (Special Edition) [Remastered]", "hail to the thief"},
	}
	for _, tt := range tests {
		got := Normalize(tt.input)
		if got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
