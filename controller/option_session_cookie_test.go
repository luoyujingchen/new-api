package controller

import "testing"

func TestNormalizeSessionCookieSameSite(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
		wantOK    bool
	}{
		{name: "strict", input: "strict", wantValue: "strict", wantOK: true},
		{name: "lax", input: "lax", wantValue: "lax", wantOK: true},
		{name: "none", input: "none", wantValue: "none", wantOK: true},
		{name: "trim and lower", input: " None ", wantValue: "none", wantOK: true},
		{name: "invalid", input: "cross-site", wantValue: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := normalizeSessionCookieSameSite(tt.input)
			if gotValue != tt.wantValue || gotOK != tt.wantOK {
				t.Fatalf(
					"normalizeSessionCookieSameSite(%q) = (%q, %v), want (%q, %v)",
					tt.input,
					gotValue,
					gotOK,
					tt.wantValue,
					tt.wantOK,
				)
			}
		})
	}
}
