package handlers

import "testing"

func TestBearerFromProtocols(t *testing.T) {
	tests := []struct {
		name      string
		protocols string
		want      string
		wantOK    bool
	}{
		{name: "bearer then token", protocols: "bearer, jwt", want: "jwt", wantOK: true},
		{name: "bearer among others", protocols: "chat, bearer, jwt", want: "jwt", wantOK: true},
		{name: "missing", protocols: ""},
		{name: "bearer without token", protocols: "bearer"},
		{name: "no bearer", protocols: "chat, jwt"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := bearerFromProtocols(test.protocols)
			if ok != test.wantOK || got != test.want {
				t.Errorf("bearerFromProtocols(%q) = %q, %v; want %q, %v", test.protocols, got, ok, test.want, test.wantOK)
			}
		})
	}
}
