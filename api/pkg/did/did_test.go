package did

import "testing"

func TestGenerateNonce(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		nonce := GenerateNonce()
		if len(nonce) != 64 {
			t.Fatalf("expected nonce of length 64, got %d", len(nonce))
		}
		if _, ok := seen[nonce]; ok {
			t.Fatal("expected unique nonces")
		}
		seen[nonce] = true
	}
}
