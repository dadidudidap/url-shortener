package handlers

import "strings"

// bearerFromProtocols reads the browser WebSocket subprotocol list, which
// carries ["bearer", token] because browsers cannot set Authorization.
func bearerFromProtocols(protocols string) (string, bool) {
	parts := strings.Split(protocols, ",")
	for i := 0; i < len(parts)-1; i++ {
		if !strings.EqualFold(strings.TrimSpace(parts[i]), "bearer") {
			continue
		}
		raw := strings.TrimSpace(parts[i+1])
		if raw == "" {
			return "", false
		}
		return raw, true
	}
	return "", false
}
