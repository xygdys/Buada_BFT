package party

import (
	"Buada_BFT/pkg/tpke"
	"strconv"

	tpk "github.com/WangZhuo2000/tpke"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/share"
)

//SigKeyGen return pk and sks, n is the number of parties, t is the threshold of combining signature
func SigKeyGen(n uint32, t uint32) ([]*share.PriShare, *share.PubPoly) {
	suit := pairing.NewSuiteBn256()
	random := suit.RandomStream()

	x := suit.G1().Scalar().Pick(random)

	// priploy
	priploy := share.NewPriPoly(suit.G2(), int(t), x, suit.RandomStream())
	// n points in ploy
	npoints := priploy.Shares(int(n))
	//pub ploy
	pubploy := priploy.Commit(suit.G2().Point().Base())
	return npoints, pubploy
}

//EncKeyGen return tpkes
func EncKeyGen(n uint32, t uint32) []*tpke.DefaultTpke {
	secretKeySet := tpk.RandomSecretKeySet(int(t - 1))
	publicKeySet := secretKeySet.PublicKeySet()
	tpkes := []*tpke.DefaultTpke{}
	for i := 0; i < int(n); i++ {
		tpke, _ := tpke.NewDefaultTpke(int(t), secretKeySet.KeyShareUsingString(strconv.Itoa(i)).Serialize(),
			publicKeySet.Serialize())

		tpkes = append(tpkes, tpke)
	}
	return tpkes
}
