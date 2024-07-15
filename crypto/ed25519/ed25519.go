// Copyright 2016 The Go Authors
// Copyright 2018 The Hyperspace Developers
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ed25519 implements the Ed25519 signature algorithm. See
// https://ed25519.cr.yp.to/.
//
// These functions are also compatible with the “Ed25519” function defined in
// RFC 8032.
package ed25519

// This code is a port of the public domain, “ref10” implementation of ed25519
// from SUPERCOP.

import (
	"bytes"
	"crypto"
	cryptorand "crypto/rand"
	"crypto/sha512"
	"io"
	"strconv"

	"github.com/kinggorrin/ptlc/crypto/ed25519/internal/edwards25519"
)

const (
	// PublicKeySize is the size, in bytes, of public keys as used in this package.
	PublicKeySize = 32
	// PrivateKeySize is the size, in bytes, of private keys as used in this package.
	PrivateKeySize = 64
	// SignatureSize is the size, in bytes, of signatures generated and verified by this package.
	SignatureSize = 64
	// AdaptorSize is the size, in bytes, of secret adaptors used in adaptor signatures
	AdaptorSize = 32
	// CurvePointSize is the size, in bytes, of points on the elliptic curve
	CurvePointSize = 32
	// CurvePointSize is the size, in bytes, of a large scalar
	ScalarSize = 32
)

// PublicKey is the type of Ed25519 public keys.
type PublicKey []byte

// PrivateKey is the type of Ed25519 private keys. It implements crypto.Signer.
type PrivateKey []byte

// Adaptor is the type of secret adaptors used in adaptor signatures
type Adaptor []byte

// CurvePoint is the byte representation of a point on the elliptic curve
type CurvePoint []byte

// Scalar is the byte represenation of a large scalar
type Scalar []byte

// Public returns the PublicKey corresponding to priv.
func (priv PrivateKey) Public() crypto.PublicKey {
	publicKey := make([]byte, PublicKeySize)
	copy(publicKey, priv[32:])
	return PublicKey(publicKey)
}

func (cp CurvePoint) toElement() edwards25519.ExtendedGroupElement {
	var element edwards25519.ExtendedGroupElement
	var pointBytes [32]byte
	copy(pointBytes[:], cp[:])
	if !element.FromBytes(&pointBytes) {
		panic("ed25519: unable to parse nonce point")
	}
	return element
}

func (cp CurvePoint) Add(point CurvePoint) CurvePoint {
	var newPointElement edwards25519.ExtendedGroupElement
	var newPoint CurvePoint
	cpElem1 := cp.toElement()
	cpElem2 := point.toElement()
	edwards25519.GeAdd(&newPointElement, &cpElem1, &cpElem2)
	newPoint = make([]byte, CurvePointSize)
	var newPointBuffer [CurvePointSize]byte
	newPointElement.ToBytes(&newPointBuffer)
	copy(newPoint[:], newPointBuffer[:])
	return newPoint
}

func (sc Scalar) Add(scalar Scalar) Scalar {
	var newScalar Scalar
	var s, s1, s2 [ScalarSize]byte
	copy(s1[:], sc[:])
	copy(s2[:], scalar[:])
	edwards25519.ScAdd(&s, &s1, &s2)
	newScalar = make([]byte, ScalarSize)
	copy(newScalar[:], s[:])
	return newScalar
}

func (sc Scalar) Subtract(scalar Scalar) Scalar {
	var newScalar Scalar
	var s, s1, s2 [ScalarSize]byte
	copy(s1[:], sc[:])
	copy(s2[:], scalar[:])
	edwards25519.ScSub(&s, &s1, &s2)
	newScalar = make([]byte, ScalarSize)
	copy(newScalar[:], s[:])
	return newScalar
}

func (sc Scalar) Multiply(scalar Scalar) Scalar {
	var newScalar Scalar
	var s, s1, s2 [ScalarSize]byte
	copy(s1[:], sc[:])
	copy(s2[:], scalar[:])
	edwards25519.ScMul(&s, &s1, &s2)
	newScalar = make([]byte, ScalarSize)
	copy(newScalar[:], s[:])
	return newScalar
}

func (sc Scalar) ToCurvePoint() CurvePoint {
	var newElem edwards25519.ExtendedGroupElement
	var newCurvePoint CurvePoint
	var scBuffer [ScalarSize]byte
	copy(scBuffer[:], sc[:])
	edwards25519.GeScalarMultBase(&newElem, &scBuffer)
	var newCurvePointBuffer [CurvePointSize]byte
	newElem.ToBytes(&newCurvePointBuffer)
	newCurvePoint = make([]byte, CurvePointSize)
	copy(newCurvePoint[:], newCurvePointBuffer[:])
	return newCurvePoint
}

// GenerateKey generates a public/private key pair using entropy from rand.
// If rand is nil, crypto/rand.Reader will be used.
func GenerateKey2(rand io.Reader) (key []byte, nonce []byte, publicKey PublicKey, privateKey PrivateKey, err error) {
	if rand == nil {
		rand = cryptorand.Reader
	}

	key = make([]byte, 32)
	nonce = make([]byte, 32)
	privateKey = make([]byte, PrivateKeySize)
	publicKey = make([]byte, PublicKeySize)
	_, err = io.ReadFull(rand, privateKey[:32])
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// https://tools.ietf.org/html/rfc8032#page-13
	// Prune the buffer: The lowest three bits of the first octet are
	// cleared, the highest bit of the last octet is cleared, and the
	// second highest bit of the last octet is set.
	digest := sha512.Sum512(privateKey[:32])
	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	var A edwards25519.ExtendedGroupElement
	var hBytes [32]byte
	copy(hBytes[:], digest[:])
	edwards25519.GeScalarMultBase(&A, &hBytes)
	var publicKeyBytes [32]byte
	A.ToBytes(&publicKeyBytes)

	copy(key[:], digest[:32])
	copy(nonce[:], digest[32:])
	copy(privateKey[32:], publicKeyBytes[:])
	copy(publicKey, publicKeyBytes[:])

	return key, nonce, publicKey, privateKey, nil
}

func Challenge(ephemeral_pk PublicKey, pk PublicKey, message []byte) Scalar {
	var digest [64]byte
	var messageDigestReduced [32]byte

	h := sha512.New()
	h.Write(pk)
	h.Write(ephemeral_pk)
	h.Write(message)
	h.Sum(digest[:0])

	scalar := make([]byte, 32)
	edwards25519.ScReduce(&messageDigestReduced, &digest)
	copy(scalar[:], messageDigestReduced[:])
	return Scalar(scalar)
}

func GenerateCurvePoint(scalar []byte) CurvePoint {
	var scalarBytes [32]byte
	var P edwards25519.ExtendedGroupElement
	copy(scalarBytes[:], scalar[:])
	edwards25519.GeScalarMultBase(&P, &scalarBytes)
	var encodedPoint [32]byte
	P.ToBytes(&encodedPoint)
	curve := make([]byte, CurvePointSize)
	copy(curve[:], encodedPoint[:])
	return CurvePoint(curve)
}

// Verify reports whether sig is a valid signature of message by publicKey. It
// will panic if len(publicKey) is not PublicKeySize.

// Verification requires sB = R + H(R,A,m)A = S
// So R = S - H(R,A,m)A
func Verify(publicKey PublicKey, message, sig []byte) bool {
	if l := len(publicKey); l != PublicKeySize {
		panic("ed25519: bad public key length: " + strconv.Itoa(l))
	}

	if len(sig) != SignatureSize || sig[63]&224 != 0 {
		return false
	}

	var A edwards25519.ExtendedGroupElement
	var publicKeyBytes [32]byte
	copy(publicKeyBytes[:], publicKey)
	if !A.FromBytes(&publicKeyBytes) {
		return false
	}
	edwards25519.FeNeg(&A.X, &A.X)
	edwards25519.FeNeg(&A.T, &A.T)

	// H(R,A,m)
	h := sha512.New()
	h.Write(sig[:32])
	h.Write(publicKey[:])
	h.Write(message)
	var digest [64]byte
	h.Sum(digest[:0])

	var hReduced [32]byte
	edwards25519.ScReduce(&hReduced, &digest)
	var hramA edwards25519.ExtendedGroupElement
	edwards25519.GeScalarMult(&hramA, &hReduced, &A)
	var hramABuffer [32]byte
	hramA.ToBytes(&hramABuffer)

	var R edwards25519.ProjectiveGroupElement
	// b is little s
	var b [32]byte
	copy(b[:], sig[32:])
	// R = - H(R,A,m)A + sB
	edwards25519.GeDoubleScalarMultVartime(&R, &hReduced, &A, &b)

	var checkR [32]byte
	R.ToBytes(&checkR)
	return bytes.Equal(sig[:32], checkR[:])
}

func ScSub(c, a, b *[32]byte) {
	edwards25519.ScSub(c, a, b)
}

func ScMul(c, a, b *[32]byte) {
	edwards25519.ScMul(c, a, b)
}

func ScAdd(c, a, b *[32]byte) {
	edwards25519.ScAdd(c, a, b)
}

func ScReduce(out *[32]byte, s *[64]byte) {
	edwards25519.ScReduce(out, s)
}

func GeAdd(A *edwards25519.ExtendedGroupElement, B *edwards25519.ExtendedGroupElement) edwards25519.ExtendedGroupElement {
	var C edwards25519.ExtendedGroupElement
	edwards25519.GeAdd(&C, A, B)
	return C
}

func GeScalarMult(a []byte, b []byte) [32]byte {
	var p edwards25519.ExtendedGroupElement
	var aBytes, bBytes [32]byte
	copy(aBytes[:], a)
	copy(bBytes[:], b)
	p.FromBytes(&bBytes)

	var h edwards25519.ExtendedGroupElement
	edwards25519.GeScalarMult(&h, &aBytes, &p)
	var hBytes [32]byte
	h.ToBytes(&hBytes)
	return hBytes
}
