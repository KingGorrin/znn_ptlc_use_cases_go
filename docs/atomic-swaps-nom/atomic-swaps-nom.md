Atomic Swaps using PTLCs
===========================

This demonstrates a single-chain atomic swap using PTLCs on the Zenon Network.

The work presented is based on the [Adaptor Signatures and Atomic Swaps from Scriptless Scripts](https://github.com/BlockstreamResearch/scriptless-scripts/blob/master/md/atomic-swap.md) and should be studied before continuing.

## Installation

The instructions below are for setting up a **devnet** with the **PTLC** support.

Follow the instructions based on the OS you're using:

- [Windows 10+](../setup-devnet-win10-x64.md)
- [Ubuntu 22.04+](../setup-devnet-linux-x64.md)

## Setup

Create a clone of the **main** branch of the [kinggorrin/znn_ptlc_use_cases_go repository](https://github.com/kinggorrin/znn_ptlc_use_cases_go.git).

```
git clone https://github.com/kinggorrin/znn_ptlc_use_cases_go.git
```

Change directory to the **znn_ptlc_use_cases_go** directory.

```
cd znn_ptlc_use_cases_go
```

## Run application

Before running the application make sure a local node is running with the generated **devnet** configuration and connect the explorer to the local node. This will give a good overview of all the onchain transactions being made by the application.

Execute the following command to start the atomic swap.

```
go run .\app\main.go
```

## Sequence diagram

The following sequence diagram shows all steps that are executed.

```mermaid
sequenceDiagram
    autonumber
    participant Ledger
    participant Alice
    participant Bob

    Alice->>Bob: Send wallet address A
    Bob->>Alice: Send wallet address B

    Note over Alice,Bob: Key generation
    Alice->>Alice: Generate key pair (a1, A1), (a2, A2), (ra, Ra) and (t, T)
    Bob->>Bob: Generate key pair (b1, B1), (b2, B2) and (rb, Rb)
    
    Alice->>Bob: Send public key (A1, A2, Ra, T)
    Bob->>Alice: Send public key (B1, B2, Rb)

    Note over Alice,Bob: Key aggregation
    Alice->>Alice: Create joint public key (A1 + B1), (A2 + B2), (Ra + T) and (Rb + T)
    Bob->>Bob: Create joint public key (A1 + B1), (A2 + B2), (Ra + T) and (Rb + T)

    Note over Alice,Bob: PTLC creation
    Alice->>Ledger: Create PTLC1: send funds, expiration and public key (A2 + B2) as Ed25519 point lock
    Ledger-->>Alice: Return PTLC1 id
    Alice->>Bob: Send PTLC1 id

    Bob->>Bob: Verify PTLC1 funds, expiration and public key

    Bob->>Ledger: Create PTLC2: send funds, expiration and public key (A1 + B1) as Ed25519 point lock
    Ledger-->>Bob: Return PTLC2 id
    Bob->>Alice: Send PTLC2 id

    Alice->>Alice: Verify PTLC2 funds, expiration and public key

    Note over Alice,Bob: Create messages
    Alice->>Alice: Create message msgA: SHA3(PTLC2 id + address A)
    Alice->>Alice: Create message msgB: SHA3(PTLC1 id + address B)
    
    Bob->>Bob: Create message msgA: SHA3(PTLC2 id + address A)
    Bob->>Bob: Create message msgB: SHA3(PTLC1 id + address B)
    
    Note over Alice,Bob: Create challenges

    Alice->>Alice: Generate challenge (c1 = SHA512((Rb + T) || (A1 + B1) || msgA))
    Alice->>Alice: Generate challenge (c2 = SHA512((Ra + T) || (A2 + B2) || msgB))
    Alice->>Bob: Send challenge (c1 * a1) and (c2 * a2)

    Bob->>Bob: Generate challenge (c1 = SHA512((Rb + T) || (A1 + B1) || msgA))
    Bob->>Bob: Generate challenge (c2 = SHA512((Ra + T) || (A2 + B2) || msgB))
    Bob->>Bob: Keep challenge ((c1a1 + c1) * b1)
    Bob->>Alice: Send challenge ((c2a2 + c2) * b2)

    Note over Alice,Bob: Create signatures

    Alice->>Bob: Send adaptor signature (s_adapt_b = (ra + c2a2b2))
    Bob->>Bob: Verify adapter signature (s_adapt_b * G == (c2 * (A2 + B2) + Ra))

    Bob->>Alice: Send adaptor signature (s_adapt_a = (rb + c1a1b1))

    Alice->>Alice: Create signature (sa = s_adapt_a + t)
    Alice->>Alice: Verify signature (sa * G == c1 * (A1 + B1) + Rb + T)
    Alice->>Alice: Create ed25519 signature (sa64 = bytes64(Rb + T, sa))

    Alice->>Ledger: Unlock PTLC2 with signature (sa64)
    Ledger-->>Alice: Send funds

    Bob->>Ledger: Get signature (sa64)
    Bob->>Bob: Extract (sa = sa64[32:])
    Bob->>Bob: Extract (t = sa - s_adapt_a)
    Bob->>Bob: Create signature (sb = s_adapt_b + t)
    Bob->>Bob: Verify signature (sb * G == c2 * (A2 + B2) + Ra + T)
    Bob->>Bob: Create ed25519 signature (sb64 = bytes64(Ra + T, sb))

    Bob->>Ledger: Unlock PTLC1 with signature (sb64)
    Ledger-->>Bob: Send funds
```