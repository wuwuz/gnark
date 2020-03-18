// Code generated by internal/gpoint DO NOT EDIT

// Most algos for points operations are taken from http://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-0.html

package bls377

import (
	"runtime"

	"github.com/consensys/gnark/ecc/bls377/fp"

	"sync"

	"github.com/consensys/gnark/ecc/bls377/fr"
	"github.com/consensys/gnark/utils/debug"
	"github.com/consensys/gnark/utils/parallel"
)

// G1Jac is a point with fp.Element coordinates
type G1Jac struct {
	X, Y, Z fp.Element
}

// G1Affine point in affine coordinates
type G1Affine struct {
	X, Y fp.Element
}

// g1JacExtended parameterized jacobian coordinates (x=X/ZZ, y=Y/ZZZ, ZZ**3=ZZZ**2)
type g1JacExtended struct {
	X, Y, ZZ, ZZZ fp.Element
}

// SetInfinity sets p to O
func (p *g1JacExtended) SetInfinity() *g1JacExtended {
	p.X.SetOne()
	p.Y.SetOne()
	p.ZZ.SetZero()
	p.ZZZ.SetZero()
	return p
}

// ToAffine sets p in affine coords
func (p *g1JacExtended) ToAffine(Q *G1Affine) *G1Affine {
	Q.X.Inverse(&p.ZZ).MulAssign(&p.X)
	Q.Y.Inverse(&p.ZZZ).MulAssign(&p.Y)
	return Q
}

// ToJac sets p in affine coords
func (p *g1JacExtended) ToJac(Q *G1Jac) *G1Jac {
	Q.X.Mul(&p.ZZ, &p.X).MulAssign(&p.ZZ)
	Q.Y.Mul(&p.ZZZ, &p.Y).MulAssign(&p.ZZZ)
	Q.Z.Set(&p.ZZZ)
	return Q
}

// mAdd
// http://www.hyperelliptic.org/EFD/g1p/auto-shortw-xyzz.html#addition-madd-2008-s
func (p *g1JacExtended) mAdd(a *G1Affine) *g1JacExtended {

	//if a is infinity return p
	if a.X.IsZero() && a.Y.IsZero() {
		return p
	}
	// p is infinity, return a
	if p.ZZ.IsZero() {
		p.X = a.X
		p.Y = a.Y
		p.ZZ.SetOne()
		p.ZZZ.SetOne()
		return p
	}

	var U2, S2, P, R, PP, PPP, Q, Q2, RR, X3, Y3 fp.Element

	// p2: a, p1: p
	U2.Mul(&a.X, &p.ZZ)
	S2.Mul(&a.Y, &p.ZZZ)
	if U2.Equal(&p.X) && S2.Equal(&p.Y) {
		return p.double(a)
	}
	P.Sub(&U2, &p.X)
	R.Sub(&S2, &p.Y)
	PP.Square(&P)
	PPP.Mul(&P, &PP)
	Q.Mul(&p.X, &PP)
	RR.Square(&R)
	X3.Sub(&RR, &PPP)
	Q2.AddAssign(&Q).AddAssign(&Q)
	p.X.Sub(&X3, &Q2)
	Y3.Sub(&Q, &p.X).MulAssign(&R)
	R.Mul(&p.Y, &PPP)
	p.Y.Sub(&Y3, &R)
	p.ZZ.MulAssign(&PP)
	p.ZZZ.MulAssign(&PPP)

	return p
}

// double point in ZZ coords
// http://www.hyperelliptic.org/EFD/g1p/auto-shortw-xyzz.html#doubling-dbl-2008-s-1
func (p *g1JacExtended) double(q *G1Affine) *g1JacExtended {

	var U, S, M, _M, Y3 fp.Element

	U.Double(&q.Y)
	p.ZZ.Square(&U)
	p.ZZZ.Mul(&U, &p.ZZ)
	S.Mul(&q.X, &p.ZZ)
	_M.Square(&q.X)
	M.Double(&_M).
		AddAssign(&_M) // -> + a, but a=0 here
	p.X.Square(&M).
		SubAssign(&S).
		SubAssign(&S)
	Y3.Sub(&S, &p.X).MulAssign(&M)
	U.Mul(&p.ZZZ, &q.Y)
	p.Y.Sub(&Y3, &U)

	return p
}

// Set set p to the provided point
func (p *G1Jac) Set(a *G1Jac) *G1Jac {
	p.X.Set(&a.X)
	p.Y.Set(&a.Y)
	p.Z.Set(&a.Z)
	return p
}

// Equal tests if two points (in Jacobian coordinates) are equal
func (p *G1Jac) Equal(a *G1Jac) bool {

	if p.Z.IsZero() && a.Z.IsZero() {
		return true
	}
	_p := G1Affine{}
	p.ToAffineFromJac(&_p)

	_a := G1Affine{}
	a.ToAffineFromJac(&_a)

	return _p.X.Equal(&_a.X) && _p.Y.Equal(&_a.Y)
}

// Equal tests if two points (in Affine coordinates) are equal
func (p *G1Affine) Equal(a *G1Affine) bool {
	return p.X.Equal(&a.X) && p.Y.Equal(&a.Y)
}

// Clone returns a copy of self
func (p *G1Jac) Clone() *G1Jac {
	return &G1Jac{
		p.X, p.Y, p.Z,
	}
}

// Neg computes -G
func (p *G1Jac) Neg(a *G1Jac) *G1Jac {
	p.Set(a)
	p.Y.Neg(&a.Y)
	return p
}

// Neg computes -G
func (p *G1Affine) Neg(a *G1Affine) *G1Affine {
	p.X.Set(&a.X)
	p.Y.Neg(&a.Y)
	return p
}

// Sub substracts two points on the curve
func (p *G1Jac) Sub(curve *Curve, a G1Jac) *G1Jac {
	a.Y.Neg(&a.Y)
	p.Add(curve, &a)
	return p
}

// ToAffineFromJac rescale a point in Jacobian coord in z=1 plane
// WARNING super slow function (due to the division)
func (p *G1Jac) ToAffineFromJac(res *G1Affine) *G1Affine {

	var bufs [3]fp.Element

	if p.Z.IsZero() {
		res.X.SetZero()
		res.Y.SetZero()
		return res
	}

	bufs[0].Inverse(&p.Z)
	bufs[2].Square(&bufs[0])
	bufs[1].Mul(&bufs[2], &bufs[0])

	res.Y.Mul(&p.Y, &bufs[1])
	res.X.Mul(&p.X, &bufs[2])

	return res
}

// ToProjFromJac converts a point from Jacobian to projective coordinates
func (p *G1Jac) ToProjFromJac() *G1Jac {
	// memalloc
	var buf fp.Element
	buf.Square(&p.Z)

	p.X.Mul(&p.X, &p.Z)
	p.Z.Mul(&p.Z, &buf)

	return p
}

func (p *G1Jac) String(curve *Curve) string {
	if p.Z.IsZero() {
		return "O"
	}
	_p := G1Affine{}
	p.ToAffineFromJac(&_p)
	_p.X.FromMont()
	_p.Y.FromMont()
	return "E([" + _p.X.String() + "," + _p.Y.String() + "]),"
}

// ToJacobian sets Q = p, Q in Jacboian, p in affine
func (p *G1Affine) ToJacobian(Q *G1Jac) *G1Jac {
	if p.X.IsZero() && p.Y.IsZero() {
		Q.Z.SetZero()
		Q.X.SetOne()
		Q.Y.SetOne()
		return Q
	}
	Q.Z.SetOne()
	Q.X.Set(&p.X)
	Q.Y.Set(&p.Y)
	return Q
}

func (p *G1Affine) String(curve *Curve) string {
	var x, y fp.Element
	x.Set(&p.X)
	y.Set(&p.Y)
	return "E([" + x.FromMont().String() + "," + y.FromMont().String() + "]),"
}

// IsInfinity checks if the point is infinity (in affine, it's encoded as (0,0))
func (p *G1Affine) IsInfinity() bool {
	return p.X.IsZero() && p.Y.IsZero()
}

// Add point addition in montgomery form
// no assumptions on z
// Note: calling Add with p.Equal(a) produces [0, 0, 0], call p.Double() instead
// https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-3.html#addition-add-2007-bl
func (p *G1Jac) Add(curve *Curve, a *G1Jac) *G1Jac {
	// p is infinity, return a
	if p.Z.IsZero() {
		p.Set(a)
		return p
	}

	// a is infinity, return p
	if a.Z.IsZero() {
		return p
	}

	// get some Element from our pool
	var Z1Z1, Z2Z2, U1, U2, S1, S2, H, I, J, r, V fp.Element

	// Z1Z1 = a.Z ^ 2
	Z1Z1.Square(&a.Z)

	// Z2Z2 = p.Z ^ 2
	Z2Z2.Square(&p.Z)

	// U1 = a.X * Z2Z2
	U1.Mul(&a.X, &Z2Z2)

	// U2 = p.X * Z1Z1
	U2.Mul(&p.X, &Z1Z1)

	// S1 = a.Y * p.Z * Z2Z2
	S1.Mul(&a.Y, &p.Z).
		MulAssign(&Z2Z2)

	// S2 = p.Y * a.Z * Z1Z1
	S2.Mul(&p.Y, &a.Z).
		MulAssign(&Z1Z1)

	// if p == a, we double instead
	if U1.Equal(&U2) && S1.Equal(&S2) {
		return p.Double()
	}

	// H = U2 - U1
	H.Sub(&U2, &U1)

	// I = (2*H)^2
	I.Double(&H).
		Square(&I)

	// J = H*I
	J.Mul(&H, &I)

	// r = 2*(S2-S1)
	r.Sub(&S2, &S1).Double(&r)

	// V = U1*I
	V.Mul(&U1, &I)

	// res.X = r^2-J-2*V
	p.X.Square(&r).
		SubAssign(&J).
		SubAssign(&V).
		SubAssign(&V)

	// res.Y = r*(V-X3)-2*S1*J
	p.Y.Sub(&V, &p.X).
		MulAssign(&r)
	S1.MulAssign(&J).Double(&S1)
	p.Y.SubAssign(&S1)

	// res.Z = ((a.Z+p.Z)^2-Z1Z1-Z2Z2)*H
	p.Z.AddAssign(&a.Z)
	p.Z.Square(&p.Z).
		SubAssign(&Z1Z1).
		SubAssign(&Z2Z2).
		MulAssign(&H)

	return p
}

// AddMixed point addition in montgomery form
// assumes a is in affine coordinates (i.e a.z == 1)
// https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-3.html#addition-add-2007-bl
// http://www.hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-0.html#addition-madd-2007-bl
func (p *G1Jac) AddMixed(a *G1Affine) *G1Jac {

	//if a is infinity return p
	if a.X.IsZero() && a.Y.IsZero() {
		return p
	}
	// p is infinity, return a
	if p.Z.IsZero() {
		p.X = a.X
		p.Y = a.Y
		// p.Z.Set(&curve.g1sZero.X)
		p.Z.SetOne()
		return p
	}

	// get some Element from our pool
	var Z1Z1, U2, S2, H, HH, I, J, r, V fp.Element

	// Z1Z1 = p.Z ^ 2
	Z1Z1.Square(&p.Z)

	// U2 = a.X * Z1Z1
	U2.Mul(&a.X, &Z1Z1)

	// S2 = a.Y * p.Z * Z1Z1
	S2.Mul(&a.Y, &p.Z).
		MulAssign(&Z1Z1)

	// if p == a, we double instead
	if U2.Equal(&p.X) && S2.Equal(&p.Y) {
		return p.Double()
	}

	// H = U2 - p.X
	H.Sub(&U2, &p.X)
	HH.Square(&H)

	// I = 4*HH
	I.Double(&HH).Double(&I)

	// J = H*I
	J.Mul(&H, &I)

	// r = 2*(S2-Y1)
	r.Sub(&S2, &p.Y).Double(&r)

	// V = X1*I
	V.Mul(&p.X, &I)

	// res.X = r^2-J-2*V
	p.X.Square(&r).
		SubAssign(&J).
		SubAssign(&V).
		SubAssign(&V)

	// res.Y = r*(V-X3)-2*Y1*J
	J.MulAssign(&p.Y).Double(&J)
	p.Y.Sub(&V, &p.X).
		MulAssign(&r)
	p.Y.SubAssign(&J)

	// res.Z =  (p.Z+H)^2-Z1Z1-HH
	p.Z.AddAssign(&H)
	p.Z.Square(&p.Z).
		SubAssign(&Z1Z1).
		SubAssign(&HH)

	return p
}

// Double doubles a point in Jacobian coordinates
// https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-3.html#doubling-dbl-2007-bl
func (p *G1Jac) Double() *G1Jac {
	// get some Element from our pool
	var XX, YY, YYYY, ZZ, S, M, T fp.Element

	// XX = a.X^2
	XX.Square(&p.X)

	// YY = a.Y^2
	YY.Square(&p.Y)

	// YYYY = YY^2
	YYYY.Square(&YY)

	// ZZ = Z1^2
	ZZ.Square(&p.Z)

	// S = 2*((X1+YY)^2-XX-YYYY)
	S.Add(&p.X, &YY)
	S.Square(&S).
		SubAssign(&XX).
		SubAssign(&YYYY).
		Double(&S)

	// M = 3*XX+a*ZZ^2
	M.Double(&XX).AddAssign(&XX)

	// res.Z = (Y1+Z1)^2-YY-ZZ
	p.Z.AddAssign(&p.Y).
		Square(&p.Z).
		SubAssign(&YY).
		SubAssign(&ZZ)

	// T = M2-2*S && res.X = T
	T.Square(&M)
	p.X = T
	T.Double(&S)
	p.X.SubAssign(&T)

	// res.Y = M*(S-T)-8*YYYY
	p.Y.Sub(&S, &p.X).
		MulAssign(&M)
	YYYY.Double(&YYYY).Double(&YYYY).Double(&YYYY)
	p.Y.SubAssign(&YYYY)

	return p
}

// ScalarMul multiplies a by scalar
// algorithm: a special case of Pippenger described by Bootle:
// https://jbootle.github.io/Misc/pippenger.pdf
func (p *G1Jac) ScalarMul(curve *Curve, a *G1Jac, scalar fr.Element) *G1Jac {
	// see MultiExp and pippenger documentation for more details about these constants / variables
	const s = 4
	const b = s
	const TSize = (1 << b) - 1
	var T [TSize]G1Jac
	computeT := func(T []G1Jac, t0 *G1Jac) {
		T[0].Set(t0)
		for j := 1; j < (1<<b)-1; j = j + 2 {
			T[j].Set(&T[j/2]).Double()
			T[j+1].Set(&T[(j+1)/2]).Add(curve, &T[j/2])
		}
	}
	return p.pippenger(curve, []G1Jac{*a}, []fr.Element{scalar}, s, b, T[:], computeT)
}

// ScalarMulByGen multiplies curve.g1Gen by scalar
// algorithm: a special case of Pippenger described by Bootle:
// https://jbootle.github.io/Misc/pippenger.pdf
func (p *G1Jac) ScalarMulByGen(curve *Curve, scalar fr.Element) *G1Jac {
	computeT := func(T []G1Jac, t0 *G1Jac) {}
	return p.pippenger(curve, []G1Jac{curve.g1Gen}, []fr.Element{scalar}, sGen, bGen, curve.tGenG1[:], computeT)
}

func (p *G1Jac) MultiExpFormer(curve *Curve, points []G1Affine, scalars []fr.Element) chan G1Jac {
	debug.Assert(len(scalars) == len(points))
	chRes := make(chan G1Jac, 1)
	// call windowed multi exp if input not large enough
	// we may want to force the API user to call the proper method in the first place
	const minPoints = 50 // under 50 points, the windowed multi exp performs better
	if len(scalars) <= minPoints {
		_points := make([]G1Jac, len(points))
		for i := 0; i < len(points); i++ {
			points[i].ToJacobian(&_points[i])
		}
		go func() {
			p.WindowedMultiExp(curve, _points, scalars)
			chRes <- *p
		}()
		return chRes

	}
	// compute nbCalls and nbPointsPerBucket as a function of available CPUs
	const chunkSize = 64
	const totalSize = chunkSize * fr.ElementLimbs
	var nbBits, nbCalls uint64
	nbPoints := len(scalars)
	nbPointsPerBucket := 20 // empirical parameter to chose nbBits
	// set nbBbits and nbCalls
	nbBits = 0
	for len(scalars)/(1<<nbBits) >= nbPointsPerBucket {
		nbBits++
	}
	nbCalls = totalSize / nbBits
	if totalSize%nbBits > 0 {
		nbCalls++
	}
	const useAllCpus = false
	// if we need to use all CPUs
	if useAllCpus {
		nbCpus := uint64(runtime.NumCPU())
		// goal here is to have at least as many calls as number of go routine we're allowed to spawn
		for nbCalls < nbCpus && nbPointsPerBucket < nbPoints {
			nbBits = 0
			for len(scalars)/(1<<nbBits) >= nbPointsPerBucket {
				nbBits++
			}
			nbCalls = totalSize / nbBits
			if totalSize%nbBits > 0 {
				nbCalls++
			}
			nbPointsPerBucket *= 2
		}
	}

	// result (1 per go routine)
	tmpRes := make([]chan G1Jac, nbCalls)
	chIndices := make([]chan struct{}, nbCalls)
	indices := make([][][]int, nbCalls)
	for i := 0; i < int(nbCalls); i++ {
		tmpRes[i] = make(chan G1Jac, 1)
		chIndices[i] = make(chan struct{}, 1)
		indices[i] = make([][]int, 0, 1<<nbBits)
		for j := 0; j < len(indices[i]); j++ {
			indices[i][j] = make([]int, 0, nbPointsPerBucket)
		}
	}

	work := func(iStart, iEnd int) {
		chunks := make([]uint64, nbBits)
		offsets := make([]uint64, nbBits)
		for i := uint64(iStart); i < uint64(iEnd); i++ {
			start := i * nbBits
			debug.Assert(start != totalSize)
			var counter uint64
			for j := start; counter < nbBits && (j < totalSize); j++ {
				chunks[counter] = j / chunkSize
				offsets[counter] = j % chunkSize
				counter++
			}
			c := 1 << counter
			indices[i] = make([][]int, c-1)
			var l uint64
			for j := 0; j < nbPoints; j++ {
				var index uint64
				for k := uint64(0); k < counter; k++ {
					l = scalars[j][chunks[k]] >> offsets[k]
					l &= 1
					l <<= k
					index += l
				}
				if index != 0 {
					indices[i][index-1] = append(indices[i][index-1], j)
				}
			}
			chIndices[i] <- struct{}{}
			close(chIndices[i])
		}
	}
	parallel.ExecuteAsyncReverse(0, int(nbCalls), work, false)

	// now we have the indices, let's compute what's inside

	debug.Assert(nbCalls > 1)
	parallel.ExecuteAsyncReverse(0, int(nbCalls), func(start, end int) {
		for i := start; i < end; i++ {
			var res G1Jac
			sum := curve.g1Infinity
			<-chIndices[i]
			for j := len(indices[i]) - 1; j >= 0; j-- {
				for k := 0; k < len(indices[i][j]); k++ {
					sum.AddMixed(&points[indices[i][j][k]])
				}
				res.Add(curve, &sum)
			}
			tmpRes[i] <- res
			close(tmpRes[i])
		}
	}, false)

	go func() {
		p.Set(&curve.g1Infinity)
		debug.Assert(len(tmpRes)-2 >= 0)
		for i := len(tmpRes) - 1; i >= 0; i-- {
			for j := uint64(0); j < nbBits; j++ {
				p.Double()
			}
			r := <-tmpRes[i]
			p.Add(curve, &r)
		}
		chRes <- *p
	}()
	return chRes
}

// MultiExp complexity O(n)
func (p *G1Jac) MultiExp(curve *Curve, points []G1Affine, scalars []fr.Element) chan G1Jac {
	nbPoints := len(points)
	debug.Assert(nbPoints == len(scalars))

	chRes := make(chan G1Jac, 1)

	// under 50 points, the windowed multi exp performs better
	const minPoints = 50
	if nbPoints <= minPoints {
		_points := make([]G1Jac, len(points))
		for i := 0; i < len(points); i++ {
			points[i].ToJacobian(&_points[i])
		}
		go func() {
			p.WindowedMultiExp(curve, _points, scalars)
			chRes <- *p
		}()
		return chRes
	}

	// empirical values
	var nbChunks, chunkSize int
	var mask uint64
	if nbPoints <= 10000 {
		chunkSize = 8
	} else if nbPoints <= 80000 {
		chunkSize = 11
	} else if nbPoints <= 400000 {
		chunkSize = 13
	} else if nbPoints <= 800000 {
		chunkSize = 14
	} else {
		chunkSize = 16
	}

	const sizeScalar = fr.ElementLimbs * 64

	var bitsForTask [][]int
	if sizeScalar%chunkSize == 0 {
		counter := sizeScalar - 1
		nbChunks = sizeScalar / chunkSize
		bitsForTask = make([][]int, nbChunks)
		for i := 0; i < nbChunks; i++ {
			bitsForTask[i] = make([]int, chunkSize)
			for j := 0; j < chunkSize; j++ {
				bitsForTask[i][j] = counter
				counter--
			}
		}
	} else {
		counter := sizeScalar - 1
		nbChunks = sizeScalar/chunkSize + 1
		bitsForTask = make([][]int, nbChunks)
		for i := 0; i < nbChunks; i++ {
			if i < nbChunks-1 {
				bitsForTask[i] = make([]int, chunkSize)
			} else {
				bitsForTask[i] = make([]int, sizeScalar%chunkSize)
			}
			for j := 0; j < chunkSize && counter >= 0; j++ {
				bitsForTask[i][j] = counter
				counter--
			}
		}
	}

	accumulators := make([]G1Jac, nbChunks)
	chIndices := make([]chan struct{}, nbChunks)
	chPoints := make([]chan struct{}, nbChunks)
	for i := 0; i < nbChunks; i++ {
		chIndices[i] = make(chan struct{}, 1)
		chPoints[i] = make(chan struct{}, 1)
	}

	mask = (1 << chunkSize) - 1
	nbPointsPerSlots := nbPoints / int(mask)
	// [][] is more efficient than [][][] for storage, elements are accessed via i*nbChunks+k
	indices := make([][]int, int(mask)*nbChunks)
	for i := 0; i < int(mask)*nbChunks; i++ {
		indices[i] = make([]int, 0, nbPointsPerSlots)
	}

	// if chunkSize=8, nbChunks=32 (the scalars are chunkSize*nbChunks bits long)
	// for each 32 chunk, there is a list of 2**8=256 list of indices
	// for the i-th chunk, accumulateIndices stores in the k-th list all the indices of points
	// for which the i-th chunk of 8 bits is equal to k
	accumulateIndices := func(cpuID, nbTasks, n int) {
		for i := 0; i < nbTasks; i++ {
			task := cpuID + i*n
			idx := task*int(mask) - 1
			for j := 0; j < nbPoints; j++ {
				val := 0
				for k := 0; k < len(bitsForTask[task]); k++ {
					val = val << 1
					c := bitsForTask[task][k] / int(64)
					o := bitsForTask[task][k] % int(64)
					b := (scalars[j][c] >> o) & 1
					val += int(b)
				}
				if val != 0 {
					indices[idx+int(val)] = append(indices[idx+int(val)], j)
				}
			}
			chIndices[task] <- struct{}{}
			close(chIndices[task])
		}
	}

	// if chunkSize=8, nbChunks=32 (the scalars are chunkSize*nbChunks bits long)
	// for each chunk, sum up elements in index 0, add to current result, sum up elements
	// in index 1, add to current result, etc, up to 255=2**8-1
	accumulatePoints := func(cpuID, nbTasks, n int) {
		for i := 0; i < nbTasks; i++ {
			var tmp g1JacExtended
			var _tmp G1Jac
			task := cpuID + i*n

			// init points
			tmp.SetInfinity()
			accumulators[task].Set(&curve.g1Infinity)

			// wait for indices to be ready
			<-chIndices[task]

			for j := int(mask - 1); j >= 0; j-- {
				for _, k := range indices[task*int(mask)+j] {
					tmp.mAdd(&points[k])
				}
				tmp.ToJac(&_tmp)
				accumulators[task].Add(curve, &_tmp)
			}
			chPoints[task] <- struct{}{}
			close(chPoints[task])
		}
	}

	// double and add algo to collect all small reductions
	reduce := func() {
		var res G1Jac
		res.Set(&curve.g1Infinity)
		for i := 0; i < nbChunks; i++ {
			for j := 0; j < len(bitsForTask[i]); j++ {
				res.Double()
			}
			<-chPoints[i]
			res.Add(curve, &accumulators[i])
		}
		p.Set(&res)
		chRes <- *p
	}

	nbCpus := runtime.NumCPU()
	nbTasksPerCpus := nbChunks / nbCpus
	remainingTasks := nbChunks % nbCpus
	for i := 0; i < nbCpus; i++ {
		if remainingTasks > 0 {
			go accumulateIndices(i, nbTasksPerCpus+1, nbCpus)
			go accumulatePoints(i, nbTasksPerCpus+1, nbCpus)
			remainingTasks--
		} else {
			go accumulateIndices(i, nbTasksPerCpus, nbCpus)
			go accumulatePoints(i, nbTasksPerCpus, nbCpus)
		}
	}

	go reduce()

	return chRes
}

// WindowedMultiExp set p = scalars[0]*points[0] + ... + scalars[n]*points[n]
// assume: scalars in non-Montgomery form!
// assume: len(points)==len(scalars)>0, len(scalars[i]) equal for all i
// algorithm: a special case of Pippenger described by Bootle:
// https://jbootle.github.io/Misc/pippenger.pdf
// uses all availables runtime.NumCPU()
func (p *G1Jac) WindowedMultiExp(curve *Curve, points []G1Jac, scalars []fr.Element) *G1Jac {
	var lock sync.Mutex
	parallel.Execute(0, len(points), func(start, end int) {
		var t G1Jac
		t.multiExp(curve, points[start:end], scalars[start:end])
		lock.Lock()
		p.Add(curve, &t)
		lock.Unlock()
	}, false)
	return p
}

// multiExp set p = scalars[0]*points[0] + ... + scalars[n]*points[n]
// assume: scalars in non-Montgomery form!
// assume: len(points)==len(scalars)>0, len(scalars[i]) equal for all i
// algorithm: a special case of Pippenger described by Bootle:
// https://jbootle.github.io/Misc/pippenger.pdf
func (p *G1Jac) multiExp(curve *Curve, points []G1Jac, scalars []fr.Element) *G1Jac {
	const s = 4 // s from Bootle, we choose s divisible by scalar bit length
	const b = s // b from Bootle, we choose b equal to s
	// WARNING! This code breaks if you switch to b!=s
	// Because we chose b=s, each set S_i from Bootle is simply the set of points[i]^{2^j} for each j in [0:s]
	// This choice allows for simpler code
	// If you want to use b!=s then the S_i from Bootle are different
	const TSize = (1 << b) - 1 // TSize is size of T_i sets from Bootle, equal to 2^b - 1
	// Store only one set T_i at a time---don't store them all!
	var T [TSize]G1Jac // a set T_i from Bootle, the set of g^j for j in [1:2^b] for some choice of g
	computeT := func(T []G1Jac, t0 *G1Jac) {
		T[0].Set(t0)
		for j := 1; j < (1<<b)-1; j = j + 2 {
			T[j].Set(&T[j/2]).Double()
			T[j+1].Set(&T[(j+1)/2]).Add(curve, &T[j/2])
		}
	}
	return p.pippenger(curve, points, scalars, s, b, T[:], computeT)
}

// algorithm: a special case of Pippenger described by Bootle:
// https://jbootle.github.io/Misc/pippenger.pdf
func (p *G1Jac) pippenger(curve *Curve, points []G1Jac, scalars []fr.Element, s, b uint64, T []G1Jac, computeT func(T []G1Jac, t0 *G1Jac)) *G1Jac {
	var t, selectorIndex, ks int
	var selectorMask, selectorShift, selector uint64

	t = fr.ElementLimbs * 64 / int(s) // t from Bootle, equal to (scalar bit length) / s
	selectorMask = (1 << b) - 1       // low b bits are 1
	morePoints := make([]G1Jac, t)    // morePoints is the set of G'_k points from Bootle
	for k := 0; k < t; k++ {
		morePoints[k].Set(&curve.g1Infinity)
	}
	for i := 0; i < len(points); i++ {
		// compute the set T_i from Bootle: all possible combinations of elements from S_i from Bootle
		computeT(T, &points[i])
		// for each morePoints: find the right T element and add it
		for k := 0; k < t; k++ {
			ks = k * int(s)
			selectorIndex = ks / 64
			selectorShift = uint64(ks - (selectorIndex * 64))
			selector = (scalars[i][selectorIndex] & (selectorMask << selectorShift)) >> selectorShift
			if selector != 0 {
				morePoints[k].Add(curve, &T[selector-1])
			}
		}
	}
	// combine morePoints to get the final result
	p.Set(&morePoints[t-1])
	for k := t - 2; k >= 0; k-- {
		for j := uint64(0); j < s; j++ {
			p.Double()
		}
		p.Add(curve, &morePoints[k])
	}
	return p
}
