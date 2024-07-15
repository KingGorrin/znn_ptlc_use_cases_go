package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/ignition-pillar/go-zdk/client"
	"github.com/ignition-pillar/go-zdk/utils"
	signer "github.com/ignition-pillar/go-zdk/wallet"
	"github.com/ignition-pillar/go-zdk/zdk"
	"github.com/kinggorrin/ptlc/crypto/ed25519"
	"github.com/tyler-smith/go-bip39"
	commoncrypto "github.com/zenon-network/go-zenon/common/crypto"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/wallet"
)

func party_alice(sender chan<- string, receiver <-chan string, wg *sync.WaitGroup) {
	fmt.Printf("Alice: Start\n")

	// Setup wallet
	mnemonic := "route become dream access impulse price inform obtain engage ski believe awful absent pig thing vibrant possible exotic flee pepper marble rural fire fancy"
	ks, _ := keyStoreFromMnemonic(mnemonic)
	_, kp, _ := ks.DeriveForIndexPath(0)
	ksigner := signer.NewSigner(kp)

	// Say hello
	sender <- "Hello from Alice"
	fmt.Printf("Alice: %v\n", <-receiver)

	// Send address
	fmt.Printf("Alice: Send wallet addressA\n")
	addressA := kp.Address
	sender <- addressA.String()

	// Receive address
	fmt.Printf("Alice: Receive wallet addressB\n")
	addressB := types.ParseAddressPanic(<-receiver)

	rpc, err := client.NewClient(client.DefaultUrl, client.ChainIdentifier(321))
	if err != nil {
		log.Fatal(err)
	}
	z := zdk.NewZdk(rpc)

	currentFrontierMomentum, err := z.Ledger.GetFrontierMomentum()
	if err != nil {
		log.Fatal(err)
	}
	currentTime := currentFrontierMomentum.TimestampUnix
	expirationTime := currentTime + (10 * 60 * 60) // convert to seconds

	// Generate keys
	fmt.Printf("Alice: Generate key pair (a1, A1), (a2, A2), (ra, Ra) and (t, T)\n")
	a1, _, A1, _, _ := ed25519.GenerateKey2(nil)
	a2, _, A2, _, _ := ed25519.GenerateKey2(nil)
	ra, _, Ra, _, _ := ed25519.GenerateKey2(nil)
	t, _, T, _, _ := ed25519.GenerateKey2(nil)

	// Send public keys
	fmt.Printf("Alice: Send public key (A1, A2, Ra, T)\n")
	sender <- hex.EncodeToString(A1)
	sender <- hex.EncodeToString(A2)
	sender <- hex.EncodeToString(Ra)
	sender <- hex.EncodeToString(T)

	// Receive public keys
	fmt.Printf("Alice: Receive public key (B1, B2, Rb)\n")
	B1Bytes, _ := hex.DecodeString(<-receiver)
	B1 := ed25519.PublicKey(B1Bytes)
	B2Bytes, _ := hex.DecodeString(<-receiver)
	B2 := ed25519.PublicKey(B2Bytes)
	RbBytes, _ := hex.DecodeString(<-receiver)
	Rb := ed25519.PublicKey(RbBytes)

	// Key aggregation
	fmt.Printf("Alice: Create joint public key (A1 + B1), (A2 + B2), (Ra + T) and (Rb + T)\n")
	AB1 := ed25519.PublicKey(ed25519.CurvePoint(A1).Add(ed25519.CurvePoint(B1)))
	AB2 := ed25519.PublicKey(ed25519.CurvePoint(A2).Add(ed25519.CurvePoint(B2)))
	RaT := ed25519.PublicKey(ed25519.CurvePoint(Ra).Add(ed25519.CurvePoint(T)))
	RbT := ed25519.PublicKey(ed25519.CurvePoint(Rb).Add(ed25519.CurvePoint(T)))

	// Create ptlc
	fmt.Printf("Alice: Create PTLC1: send funds, expiration and public key (A2 + B2) as Ed25519 point lock\n")
	ptlc1AB, _ := z.Embedded.Ptlc.Create(
		types.ZnnTokenStandard,
		big.NewInt(1000000000),
		int64(expirationTime),
		0,
		AB2)
	pltc1, err := utils.Send(z,
		ptlc1AB,
		ksigner,
		true)
	if err != nil {
		log.Fatal(err)
	}
	ptlc1Id := pltc1.Hash

	// Wait 2 momentums
	fmt.Printf("Alice: Wait 2 momentums\n")
	time.Sleep(time.Second * 10 * 2)

	// Send ptlc id
	fmt.Printf("Alice: Send ptlc1 id\n")
	sender <- hex.EncodeToString(ptlc1Id.Bytes())

	// Receive ptlc id
	fmt.Printf("Alice: Receive ptlc2 id\n")
	ptlc2Id := types.HexToHashPanic(<-receiver)

	// Create messages
	fmt.Printf("Alice: Create message msgA: SHA3(PTLC2 id + addressA)\n")
	msgA := commoncrypto.Hash(append(ptlc2Id.Bytes()[:], addressA.Bytes()...))

	fmt.Printf("Alice: Create message msgB: SHA3(PTLC1 id + addressB)\n")
	msgB := commoncrypto.Hash(append(ptlc1Id.Bytes()[:], addressB.Bytes()...))

	// Generates challenges
	fmt.Printf("Alice: Generate challenge (c1 = SHA512((Rb + T) || (A1 + B1) || msgA))\n")
	c1 := ed25519.Challenge(AB1, RbT, msgA)
	fmt.Printf("Alice: Generate challenge (c2 = SHA512((Ra + T) || (A2 + B2) || msgB))\n")
	c2 := ed25519.Challenge(AB2, RaT, msgB)

	c1a1 := c1.Multiply(ed25519.Scalar(a1[:32]))
	c2a2 := c2.Multiply(ed25519.Scalar(a2[:32]))

	// Sends challenges
	fmt.Printf("Alice: Send challenge (c1 * a1) and (c2 * a2)\n")
	sender <- hex.EncodeToString(c1a1)
	sender <- hex.EncodeToString(c2a2)

	// Receive challenges
	fmt.Printf("Alice: Receive challenge ((c2a2 + c2) * b2)\n")
	c2a2b2Bytes, _ := hex.DecodeString(<-receiver)
	c2a2b2 := ed25519.Scalar(c2a2b2Bytes)

	// Create signatures

	// Sends adaptor signature to Bob
	fmt.Printf("Alice: Send adaptor signature (s_adapt_b = (ra + c2a2b2))\n")
	s_adapt_b := c2a2b2.Add(ed25519.Scalar(ra[:32]))
	sender <- hex.EncodeToString(s_adapt_b)

	// Receive adaptor signature
	fmt.Printf("Alice: Receive adaptor signature (s_adapt_a = (rb + c1a1b1))\n")
	s_adapt_a_bytes, _ := hex.DecodeString(<-receiver)
	s_adapt_a := ed25519.Scalar(s_adapt_a_bytes)

	// Alice is now able to publish her full signature
	fmt.Printf("Alice: Create signature (sa = s_adapt_a + t)\n")
	sa := s_adapt_a.Add(t)

	// Verify signature
	fmt.Printf("Alice: Verify signature (sa * G == c1 * (A1 + B1) + Rb + T)\n")
	c1AB1 := ed25519.GeScalarMult(c1, AB1)
	c1AB1RbT := ed25519.CurvePoint(c1AB1[:]).Add(ed25519.CurvePoint(RbT))

	if !bytes.Equal(ed25519.GenerateCurvePoint(sa), c1AB1RbT) {
		panic("Alice: signature is invalid")
	}

	// Create ed25519 signature
	fmt.Printf("Alice: Create ed25519 signature (sa64 = bytes64(Rb + T, sa))\n")
	sa64 := make([]byte, 64)
	copy(sa64[:32], RbT[:32])
	copy(sa64[32:], sa[:32])

	if !ed25519.Verify(AB1, msgA, sa64) {
		panic("Alice: signature is invalid")
	}

	// Unlock PTLC
	fmt.Printf("Alice: Unlock PTLC2 with signature (sa64)\n")
	unlockPtlc2AB, _ := z.Embedded.Ptlc.Unlock(ptlc2Id, sa64)
	_, err2 := utils.Send(z,
		unlockPtlc2AB,
		ksigner,
		true)
	if err2 != nil {
		log.Fatal(err2)
	}

	// Wait 2 momentums
	fmt.Printf("Alice: Wait 2 momentums\n")
	time.Sleep(time.Second * 10 * 2)

	// Sends signature to Bob
	// Bob should actually retrieve this onchain, but this is easier
	sender <- hex.EncodeToString(sa)

	fmt.Printf("Alice: End\n")
	wg.Done()
}

func party_bob(sender chan<- string, receiver <-chan string, wg *sync.WaitGroup) {
	fmt.Printf("Bob: Start\n")

	// Setup wallet
	mnemonic := "alone emotion announce page spend eager middle lucky frame craft junk artefact upper finger drive corn version slot blade picnic festival wealth critic silver"
	ks, _ := keyStoreFromMnemonic(mnemonic)
	_, kp, _ := ks.DeriveForIndexPath(0)
	ksigner := signer.NewSigner(kp)

	// Say hello
	fmt.Printf("Bob: %v\n", <-receiver)
	sender <- "Hello from Bob"

	// Receive address
	fmt.Printf("Bob: Receive wallet addressA\n")
	addressA := types.ParseAddressPanic(<-receiver)

	// Send address
	fmt.Printf("Bob: Send wallet addressB\n")
	addressB := kp.Address
	sender <- addressB.String()

	// Connect client
	rpc, err := client.NewClient(client.DefaultUrl, client.ChainIdentifier(321))
	if err != nil {
		log.Fatal(err)
	}
	z := zdk.NewZdk(rpc)

	currentFrontierMomentum, err := z.Ledger.GetFrontierMomentum()
	if err != nil {
		log.Fatal(err)
	}
	currentTime := currentFrontierMomentum.TimestampUnix
	expirationTime := currentTime + (10 * 60 * 60) // convert to seconds

	// Generate keys
	fmt.Printf("Bob: Generate key pair (b1, B1), (b2, B2) and (rb, Rb)\n")
	b1, _, B1, _, _ := ed25519.GenerateKey2(nil)
	b2, _, B2, _, _ := ed25519.GenerateKey2(nil)
	rb, _, Rb, _, _ := ed25519.GenerateKey2(nil)

	// Receive public keys
	fmt.Printf("Bob: Receive public key (A1, A2, Ra, T)\n")
	A1Bytes, _ := hex.DecodeString(<-receiver)
	A1 := ed25519.PublicKey(A1Bytes)
	A2Bytes, _ := hex.DecodeString(<-receiver)
	A2 := ed25519.PublicKey(A2Bytes)
	RaBytes, _ := hex.DecodeString(<-receiver)
	Ra := ed25519.PublicKey(RaBytes)
	TBytes, _ := hex.DecodeString(<-receiver)
	T := ed25519.PublicKey(TBytes)

	// Send public key
	fmt.Printf("Bob: Send public key (B1, B2, Rb)\n")
	sender <- hex.EncodeToString(B1)
	sender <- hex.EncodeToString(B2)
	sender <- hex.EncodeToString(Rb)

	// Key aggregation
	fmt.Printf("Bob: Create joint public key (A1 + B1), (A2 + B2), (Ra + T) and (Rb + T)\n")
	AB1 := ed25519.PublicKey(ed25519.CurvePoint(A1).Add(ed25519.CurvePoint(B1)))
	AB2 := ed25519.PublicKey(ed25519.CurvePoint(A2).Add(ed25519.CurvePoint(B2)))
	RaT := ed25519.PublicKey(ed25519.CurvePoint(Ra).Add(ed25519.CurvePoint(T)))
	RbT := ed25519.PublicKey(ed25519.CurvePoint(Rb).Add(ed25519.CurvePoint(T)))

	// Receive ptlc
	fmt.Printf("Bob: Receive PTLC1 id\n")
	ptlc1Id := types.HexToHashPanic(<-receiver)

	// Verify funds
	fmt.Printf("Bob: Verify PTLC1 funds, expiration and public key\n")
	// TODO: Bob should verify the ptlc

	// Create ptlc
	fmt.Printf("Bob: Create PTLC2: send funds, expiration and public key (A1 + B1) as Ed25519 point lock\n")
	ptlc2AB, _ := z.Embedded.Ptlc.Create(types.QsrTokenStandard, big.NewInt(10000000000), int64(expirationTime), 0, AB1)
	pltc2, err := utils.Send(z,
		ptlc2AB,
		ksigner,
		true)
	if err != nil {
		log.Fatal(err)
	}
	ptlc2Id := pltc2.Hash

	// Wait 2 momentums
	fmt.Printf("Bob: Wait 2 momentums\n")
	time.Sleep(time.Second * 10 * 2)

	// Send ptlc id
	fmt.Printf("Bob: Send PTLC2 id\n")
	sender <- hex.EncodeToString(pltc2.Hash.Bytes())

	// Create messages
	fmt.Printf("Bob: Create message msgA: SHA3(PTLC2 id + addressA)\n")
	msgA := commoncrypto.Hash(append(ptlc2Id.Bytes()[:], addressA.Bytes()...))
	fmt.Printf("Bob: Create message msgB: SHA3(PTLC1 id + addressB)\n")
	msgB := commoncrypto.Hash(append(ptlc1Id.Bytes()[:], addressB.Bytes()...))

	// Receive challenges
	fmt.Printf("Bob: Receive challenge (c1 * a1) and (c2 * a2)\n")
	c1a1Bytes, _ := hex.DecodeString(<-receiver)
	c1a1 := ed25519.Scalar(c1a1Bytes)
	fmt.Printf("Bob: Receive challenge (c1 * a1) and (c2 * a2)\n")
	c2a2Bytes, _ := hex.DecodeString(<-receiver)
	c2a2 := ed25519.Scalar(c2a2Bytes)

	// Generate challenges
	fmt.Printf("Bob: Generate challenge (c1 = SHA512((Rb + T) || (A1 + B1) || msgA))\n")
	c1 := ed25519.Challenge(AB1, RbT, msgA)
	fmt.Printf("Bob: Generate challenge (c2 = SHA512((Ra + T) || (A2 + B2) || msgB))\n")
	c2 := ed25519.Challenge(AB2, RaT, msgB)

	c1a1b1 := c1.Multiply(ed25519.Scalar(b1[:32])).Add(c1a1)
	c2a2b2 := c2.Multiply(ed25519.Scalar(b2[:32])).Add(c2a2)

	// Bob sends c2*(a2 + b2) to Alice but keeps c1*(a1 + b1) for now
	fmt.Printf("Bob: Send challenge ((c2a2 + c2) * b2)\n")
	sender <- hex.EncodeToString(c2a2b2)

	// Receive adapter signature
	fmt.Printf("Bob: Receive adapter signature (s_adapt_b = (ra + c2a2b2))\n")
	s_adapt_b_bytes, _ := hex.DecodeString(<-receiver)
	s_adapt_b := ed25519.Scalar(s_adapt_b_bytes)

	// Verify signature
	fmt.Printf("Bob: Verify adapter signature (s_adapt_b * G == (c2 * (A2 + B2) + Ra))\n")
	c2AB2 := ed25519.GeScalarMult(c2, AB2)
	c2AB2Ra := ed25519.CurvePoint(c2AB2[:]).Add(ed25519.CurvePoint(Ra))

	if !bytes.Equal(ed25519.GenerateCurvePoint(s_adapt_b), c2AB2Ra) {
		panic("Bob: adapter signature is invalid")
	}

	// Verification is OK so Bob is safe to send his signature Alice
	fmt.Printf("Bob: Send adaptor signature (s_adapt_a = (rb + c1a1b1))\n")
	s_adapt_a := c1a1b1.Add(rb)
	sender <- hex.EncodeToString(s_adapt_a)

	// Receive signature
	fmt.Printf("Bob: Receive signature (sa)\n")
	sa_bytes, _ := hex.DecodeString(<-receiver)
	sa := ed25519.Scalar(sa_bytes)

	// Bob can now infer `t` and build his signature
	fmt.Printf("Bob: Extract (t = sa - s_adapt_a)\n")
	t := sa.Subtract(s_adapt_a)
	fmt.Printf("Bob: Create signature (sb = s_adapt_b + t)\n")
	sb := s_adapt_b.Add(t)
	c2AB2RaT := ed25519.CurvePoint(c2AB2[:]).Add(ed25519.CurvePoint(RaT))

	fmt.Printf("Bob: Verify signature (sb * G == c2 * (A2 + B2) + Ra + T)\n")
	if !bytes.Equal(ed25519.GenerateCurvePoint(sb), c2AB2RaT) {
		panic("Bob: signature is invalid")
	}

	// Create ed25519 signature
	fmt.Printf("Bob: Create ed25519 signature (sb64 = bytes64(Ra + T, sb))\n")
	sb64 := make([]byte, 64)
	copy(sb64[:32], RaT[:32])
	copy(sb64[32:], sb[:32])

	if !ed25519.Verify(AB2, msgB, sb64) {
		panic("Bob: signature is invalid")
	}

	// Unlock PTLC
	fmt.Printf("Bob: Unlock PTLC1 with signature (sb64)\n")
	unlockPtlc1AB, _ := z.Embedded.Ptlc.Unlock(ptlc1Id, sb64)
	_, err = utils.Send(z,
		unlockPtlc1AB,
		ksigner,
		true)
	if err != nil {
		log.Fatal(err)
	}

	// Wait 2 momentums
	fmt.Printf("Bob: Wait 2 momentums\n")
	time.Sleep(time.Second * 10 * 2)

	fmt.Printf("Bob: End\n")
	wg.Done()
}

func main() {
	fmt.Println("App: Start")

	// Create a WaitGroup
	var wg sync.WaitGroup
	wg.Add(2)

	// Create channels
	c1 := make(chan string)
	c2 := make(chan string)

	go party_alice(c1, c2, &wg)
	go party_bob(c2, c1, &wg)

	wg.Wait()

	fmt.Println("App: End")
}

func keyStoreFromMnemonic(mnemonic string) (*wallet.KeyStore, error) {
	entropy, err := bip39.EntropyFromMnemonic(mnemonic)
	if err != nil {
		return nil, err
	}

	ks := &wallet.KeyStore{
		Entropy:  entropy,
		Seed:     bip39.NewSeed(mnemonic, ""),
		Mnemonic: mnemonic,
	}

	// setup base address
	if _, kp, err := ks.DeriveForIndexPath(0); err == nil {
		ks.BaseAddress = kp.Address
	} else {
		return nil, err
	}

	return ks, nil
}
