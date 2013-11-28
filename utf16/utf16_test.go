package utf16
import (
	"testing"
)

func TestEncodeAndDecode(t *testing.T) {
	original := "test123世界blaat"
	encoded := Encode(original)
	decoded := Decode(encoded)
	if decoded != original {
		t.Fatalf("Original and decoded doesn't match, %v (% x) vs. %v (% x)", original, original, decoded, decoded)
	}
}