package zkpschemes

const Groth16Tests = `


{{ template "header" . }}

package groth16_test

import (
	{{ template "import_curve" . }}
	{{ template "import_backend" . }}
	"path/filepath"
	"runtime/debug"
	"testing"

	{{if eq .Curve "BLS377"}}
	"github.com/consensys/gnark/backend/bls377/groth16"
	{{else if eq .Curve "BLS381"}}
	"github.com/consensys/gnark/backend/bls381/groth16"
	{{else if eq .Curve "BN256"}}
	"github.com/consensys/gnark/backend/bn256/groth16"
	{{end}}

	
	"github.com/consensys/gnark/utils/encoding/gob"
	constants "github.com/consensys/gnark/backend"
)


func TestCircuits(t *testing.T) {
	assert := groth16.NewAssert(t)
	matches, _ := filepath.Glob("./testdata/*.r1cs")
	for _, name := range matches {
		name = name[:len(name)-5]
		t.Log("testing circuit", name)

		good := backend.NewAssignment()
		if err := good.Read(name + ".good"); err != nil {
			t.Fatal(err)
		}
		bad := backend.NewAssignment()
		if err := bad.Read(name + ".bad"); err != nil {
			t.Fatal(err)
		}
		var r1cs backend.R1CS

		if err := gob.Read(name+".r1cs", &r1cs, curve.ID); err != nil {
			t.Fatal(err)
		}
		assert.NotSolved(&r1cs, bad)
		assert.Solved(&r1cs, good, nil)
	}
}

// test input
// TODO should probably not be here
func TestParsePublicInput(t *testing.T) {

	expectedNames := [2]string{"data", "ONE_WIRE"}

	inputOneWire := backend.NewAssignment()
	inputOneWire.Assign(constants.Public, "ONE_WIRE", 3)
	_, errOneWire := groth16.ParsePublicInput(expectedNames[:], inputOneWire)
	if errOneWire == nil {
		t.Fatal("expected ErrGotOneWire error")
	}

	inputPrivate := backend.NewAssignment()
	inputPrivate.Assign(constants.Secret, "data", 3)
	_, errPrivate := groth16.ParsePublicInput(expectedNames[:], inputPrivate)
	if errPrivate == nil {
		t.Fatal("expected ErrGotPrivateInput error")
	}

	missingInput := backend.NewAssignment()
	_, errMissing := groth16.ParsePublicInput(expectedNames[:], missingInput)
	if errMissing == nil {
		t.Fatal("expected ErrMissingAssigment")
	}

	correctInput := backend.NewAssignment()
	correctInput.Assign(constants.Public, "data", 3)
	got, err := groth16.ParsePublicInput(expectedNames[:], correctInput)
	if err != nil {
		t.Fatal(err)
	}

	expected := make([]fr.Element, 2)
	expected[0].SetUint64(3).FromMont()
	expected[1].SetUint64(1).FromMont()
	if len(got) != len(expected) {
		t.Fatal("Unexpected length for assignment")
	}
	for i := 0; i < len(got); i++ {
		if !got[i].Equal(&expected[i]) {
			t.Fatal("error public assignment")
		}
	}

}

//--------------------//
//     benches		  //
//--------------------//

func referenceCircuit() (backend.R1CS, backend.Assignments, backend.Assignments) {
	name := "./testdata/reference"
	good := backend.NewAssignment()
	if err := good.Read(name + ".good"); err != nil {
		panic(err)
	}
	bad := backend.NewAssignment()
	if err := bad.Read(name + ".bad"); err != nil {
		panic(err)
	}
	var r1cs backend.R1CS

	if err := gob.Read(name+".r1cs", &r1cs, curve.ID); err != nil {
		panic(err)
	}

	return r1cs, good, bad
}

// BenchmarkSetup is a helper to benchmark groth16.Setup on a given circuit
func BenchmarkSetup(b *testing.B) {
	r1cs, _, _ := referenceCircuit()
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var pk groth16.ProvingKey
	var vk groth16.VerifyingKey
	b.ResetTimer()

	b.Run("setup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			groth16.Setup(&r1cs, &pk, &vk)
		}
	})
}

// BenchmarkProver is a helper to benchmark groth16.Prove on a given circuit
// it will run the Setup, reset the benchmark timer and benchmark the prover
func BenchmarkProver(b *testing.B) {
	r1cs, solution, _ := referenceCircuit()
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var pk groth16.ProvingKey
	var vk groth16.VerifyingKey
	groth16.Setup(&r1cs, &pk, &vk)

	b.ResetTimer()
	b.Run("prover", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = groth16.Prove(&r1cs, &pk, solution)
		}
	})
}

// BenchmarkVerifier is a helper to benchmark groth16.Verify on a given circuit
// it will run the Setup, the Prover and reset the benchmark timer and benchmark the verifier
// the provided solution will be filtered to keep only public inputs
func BenchmarkVerifier(b *testing.B) {
	r1cs, solution, _ := referenceCircuit()
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var pk groth16.ProvingKey
	var vk groth16.VerifyingKey
	groth16.Setup(&r1cs, &pk, &vk)
	proof, err := groth16.Prove(&r1cs, &pk, solution)
	if err != nil {
		panic(err)
	}

	solution = filterOutPrivateAssignment(solution)
	b.ResetTimer()
	b.Run("verifier", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = groth16.Verify(proof, &vk, solution)
		}
	})
}

func filterOutPrivateAssignment(assignments map[string]backend.Assignment) map[string]backend.Assignment {
	toReturn := backend.NewAssignment()
	for k, v := range assignments {
		if v.IsPublic {
			toReturn[k] = v
		}
	}
	return toReturn
}

`
