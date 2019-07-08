package handler

import "testing"

// silly test for a simple function. i was getting used to
// testing in go at the time. might aswell keep it though
func TestFirstExisting(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		or     string
		exp    string
	}{
		{
			"none present",
			[]string{"one", "two", "three"}, "default",
			"one",
		},
		{
			"first missing",
			[]string{"", "two", "three"}, "default",
			"two",
		},
		{
			"all missing",
			[]string{"", "", ""}, "default",
			"default",
		},
	}
	for _, tc := range cases {
		tc := tc // pin
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actu := firstExisting(tc.or, tc.values...)
			if actu != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, actu)
			}
		})
	}
}
