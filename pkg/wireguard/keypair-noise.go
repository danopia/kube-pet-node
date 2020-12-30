package wireguard

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/curve25519"
)

// https://github.com/WireGuard/wireguard-go/blob/master/device/noise-types.go
// https://github.com/WireGuard/wireguard-go/blob/master/device/noise-helpers.go

const (
	noisePublicKeySize  = 32
	noisePrivateKeySize = 32
)

type (
	noisePublicKey  [noisePublicKeySize]byte
	noisePrivateKey [noisePrivateKeySize]byte
)

func (sk *noisePrivateKey) clamp() {
	sk[0] &= 248
	sk[31] = (sk[31] & 127) | 64
}

func newPrivateKey() (sk noisePrivateKey, err error) {
	_, err = rand.Read(sk[:])
	sk.clamp()
	return
}

func readPrivateKeyFromHex(src string) (sk noisePrivateKey, err error) {
	err = loadExactHex(sk[:], src)
	sk.clamp()
	return
}

func readPrivateKeyFromBase64(src string) (sk noisePrivateKey, err error) {
	err = loadExactBase64(sk[:], src)
	sk.clamp()
	return
}

func (sk *noisePrivateKey) publicKey() (pk noisePublicKey) {
	apk := (*[noisePublicKeySize]byte)(&pk)
	ask := (*[noisePrivateKeySize]byte)(sk)
	curve25519.ScalarBaseMult(apk, ask)
	return
}

func loadExactHex(dst []byte, src string) error {
	slice, err := hex.DecodeString(src)
	if err != nil {
		return err
	}
	if len(slice) != len(dst) {
		return errors.New("hex string does not fit the slice")
	}
	copy(dst, slice)
	return nil
}

func loadExactBase64(dst []byte, src string) error {
	slice, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		return err
	}
	if len(slice) != len(dst) {
		return errors.New("base64 string does not fit the slice")
	}
	copy(dst, slice)
	return nil
}

func (sk noisePrivateKey) IsZero() bool {
	var zero noisePrivateKey
	return sk.Equals(zero)
}

func (sk noisePrivateKey) Equals(tar noisePrivateKey) bool {
	return subtle.ConstantTimeCompare(sk[:], tar[:]) == 1
}

func (sk *noisePrivateKey) FromHex(src string) (err error) {
	err = loadExactHex(sk[:], src)
	sk.clamp()
	return
}

func (sk *noisePrivateKey) FromMaybeZeroHex(src string) (err error) {
	err = loadExactHex(sk[:], src)
	if sk.IsZero() {
		return
	}
	sk.clamp()
	return
}

func (sk noisePrivateKey) ToHex() string {
	return hex.EncodeToString(sk[:])
}

func (sk noisePrivateKey) ToBase64() string {
	return base64.StdEncoding.EncodeToString(sk[:])
}

func (key *noisePublicKey) FromHex(src string) error {
	return loadExactHex(key[:], src)
}

func (key noisePublicKey) ToHex() string {
	return hex.EncodeToString(key[:])
}

func (key noisePublicKey) ToBase64() string {
	return base64.StdEncoding.EncodeToString(key[:])
}

func (key noisePublicKey) IsZero() bool {
	var zero noisePublicKey
	return key.Equals(zero)
}

func (key noisePublicKey) Equals(tar noisePublicKey) bool {
	return subtle.ConstantTimeCompare(key[:], tar[:]) == 1
}
