package naukri

import "testing"

func TestToggleTrailingFullStop(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no trailing dot", "Experienced engineer", "Experienced engineer."},
		{"one trailing dot", "Experienced engineer.", "Experienced engineer.."},
		{"two trailing dots", "Experienced engineer..", "Experienced engineer."},
		{"trailing whitespace after dots", "Experienced engineer..  \n", "Experienced engineer."},
		{"trailing whitespace after single dot", "Experienced engineer. \t", "Experienced engineer.."},
		{"empty string", "", "."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toggleTrailingFullStop(tc.in)
			if got != tc.want {
				t.Errorf("toggleTrailingFullStop(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
