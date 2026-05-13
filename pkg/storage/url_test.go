package storage

import "testing"

func TestNormalizeCDNURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"http://cdn.example.com", "http://cdn.example.com"},
		{"http://cdn.example.com/", "http://cdn.example.com"},
		{"http://cdn.example.com/storage", "http://cdn.example.com"},
		{"http://cdn.example.com/storage/", "http://cdn.example.com"},
		{"http://178.105.131.54:9000", "http://178.105.131.54:9000"},
		{"http://178.105.131.54:9000/", "http://178.105.131.54:9000"},
		{"http://178.105.131.54:9000/storage", "http://178.105.131.54:9000"},
		{"http://178.105.131.54:9000/storage/", "http://178.105.131.54:9000"},
		{"https://cdn.example.com/storage/extra", "https://cdn.example.com/storage/extra"},
	}
	for _, tc := range cases {
		if got := normalizeCDNURL(tc.in); got != tc.want {
			t.Errorf("normalizeCDNURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
