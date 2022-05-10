package tpke

// Note: code copy from
// file1: https://github.com/DE-labtory/cleisthenes/tpke/threshold_encrpytion.go
// file2: https://github.com/DE-labtory/cleisthenes/tpke/threshold_encryption_test.go
// with Apache 2.0 license
//
// Copyright 2019 DE-labtory
//
// Modify: combine file1 and file2, add keyGen algorithm,
//		   rename interface name to avoid repetition with
//		   this project package name

import (
	"strconv"

	tpk "github.com/WangZhuo2000/tpke"
)

type SecretKey [32]byte
type PublicKey []byte
type DecryptionShare [96]byte
type CipherText []byte

type DefaultTpke struct {
	Threshold    int
	PublicKey    *tpk.PublicKey
	PublicKeySet *tpk.PublicKeySet
	SecretKey    *tpk.SecretKeyShare
	DecShares    map[string]*tpk.DecryptionShare
}

func NewDefaultTpke(th int, skStr SecretKey, pksStr PublicKey) (*DefaultTpke, error) {
	sk := tpk.NewSecretKeyFromBytes(skStr)
	sks := tpk.NewSecretKeyShare(sk)

	pks, err := tpk.NewPublicKeySetFromBytes(pksStr)
	if err != nil {
		return nil, err
	}

	return &DefaultTpke{
		Threshold:    th,
		PublicKeySet: pks,
		PublicKey:    pks.PublicKey(),
		SecretKey:    sks,
		DecShares:    make(map[string]*tpk.DecryptionShare),
	}, nil
}

func (t *DefaultTpke) AcceptDecShare(id string, decShare DecryptionShare) {
	ds := tpk.NewDecryptionShareFromBytes(decShare)
	t.DecShares[id] = ds
}

func (t *DefaultTpke) ClearDecShare() {
	t.DecShares = make(map[string]*tpk.DecryptionShare)
}

// Encrypt encrypts some byte array message.
func (t *DefaultTpke) Encrypt(msg []byte) ([]byte, error) {
	encrypted, err := t.PublicKey.Encrypt(msg)
	if err != nil {
		return nil, err
	}
	return encrypted.Serialize(), nil
}

// DecShare makes decryption share using each secret key.
func (t *DefaultTpke) DecShare(ctb CipherText) DecryptionShare {
	ct := tpk.NewCipherTextFromBytes(ctb)
	ds := t.SecretKey.DecryptShare(ct)
	return ds.Serialize()
}

// Decrypt collects decryption share, and combine it for decryption.
func (t *DefaultTpke) Decrypt(DecShares map[string]DecryptionShare, ctBytes []byte) ([]byte, error) {
	ct := tpk.NewCipherTextFromBytes(ctBytes)
	ds := make(map[string]*tpk.DecryptionShare)
	for id, decShare := range DecShares {
		ds[id] = tpk.NewDecryptionShareFromBytes(decShare)
	}
	return t.PublicKeySet.DecryptUsingStringMap(ds, ct)
}

type TPKE struct {
	id   int
	tpke *DefaultTpke
}

func NewTPKE(N int, t int) []*TPKE {
	tpkes := []*TPKE{}
	secretKeySet := tpk.RandomSecretKeySet(t)
	PublicKeySet := secretKeySet.PublicKeySet()
	for i := 0; i < N; i++ {
		tpke, _ := NewDefaultTpke(t, secretKeySet.KeyShareUsingString(strconv.Itoa(i)).Serialize(),
			PublicKeySet.Serialize())
		tpkes = append(tpkes, &TPKE{
			id:   i,
			tpke: tpke,
		})
	}
	return tpkes
}

func Enc(msg []byte, tpke *DefaultTpke) []byte {
	decshare, _ := tpke.Encrypt(msg)
	return decshare
}

func DecShare(ct []byte, tpke *DefaultTpke) []byte {
	ds := tpke.DecShare(ct)
	return ds[:]
}

func Dec(m map[int][]byte, ct []byte, tpke *DefaultTpke) []byte {
	mm := map[string]DecryptionShare{}
	for k, v := range m {
		var arr DecryptionShare
		copy(arr[:], v)
		mm[strconv.Itoa(k)] = arr
	}
	msg, _ := tpke.Decrypt(mm, ct)
	return msg
}
