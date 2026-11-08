package elfsim

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/sarchlab/akita/v5/sim/timing"
)

// Calibration declarations are independent of frozen native input manifests,
// but must describe the actual defaults used by ELF timing execution.
func TestDeclaredModelProfileMatchesRuntimeDefaults(t *testing.T) {
	data, err := os.ReadFile("../../evaluation/model-profile.json")
	if err != nil {
		t.Fatal(err)
	}
	var declared any
	if err = json.Unmarshal(data, &declared); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(DefaultTimingProfile(3490 * timing.MHz))
	if err != nil {
		t.Fatal(err)
	}
	var actual any
	if err = json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(declared, actual) {
		t.Fatalf("model-profile.json differs from runtime defaults\ndeclared=%s\nactual=%s", data, raw)
	}
}
