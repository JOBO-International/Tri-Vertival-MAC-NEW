//go:build !windows

package ki

// Ohne Windows (nur für Tests): unverschlüsselt.
func verschluesseln(b []byte) ([]byte, error) { return b, nil }
func entschluesseln(b []byte) ([]byte, error) { return b, nil }
