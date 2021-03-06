package config

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"time"

	"github.com/Sukhavati-Labs/go-miner/poc"

	"github.com/Sukhavati-Labs/go-miner/pocec"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

// genesisCoinbaseTx is the coinbase transaction for genesis block
var genesisCoinbaseTx = wire.MsgTx{
	Version: 1,
	TxIn: []*wire.TxIn{
		{
			PreviousOutPoint: wire.OutPoint{
				Hash:  wire.Hash{},
				Index: wire.MaxPrevOutIndex,
			},
			Sequence: wire.MaxTxInSequenceNum,
			Witness:  wire.TxWitness{},
		},
	},
	TxOut: []*wire.TxOut{
		{
			Value:    0x2FAF08000,
			PkScript: mustDecodeString("002018ab40cde6f4f6087e27d118717bc80c176c5c4d16d2308423e893689b8d1823"),
		},
	},
	LockTime: 0,
	Payload:  mustDecodeString("000000000000000000000000"),
}

var genesisHeader = wire.BlockHeader{
	ChainID:         mustDecodeHash("3e5c9bbb72a812303dd01b99ffe7fba755a7272a3a42abf2e983fdc2c0ec34b8"),
	Version:         1,
	Height:          0,
	Timestamp:       time.Unix(0x60572CB4, 0), // 2021-03-21 11:24:00 +0000 UTC, 1616325812 0x60572CB4
	Previous:        mustDecodeHash("0000000000000000000000000000000000000000000000000000000000000000"),
	TransactionRoot: mustDecodeHash("ada8ce6758af6b5291349f58900f87f6b0e051fb43d9f756ea54c3fb708d950f"),
	WitnessRoot:     mustDecodeHash("ada8ce6758af6b5291349f58900f87f6b0e051fb43d9f756ea54c3fb708d950f"),
	ProposalRoot:    mustDecodeHash("9663440551fdcd6ada50b1fa1b0003d19bc7944955820b54ab569eb9a7ab7999"),
	Target:          hexToBigInt("44814ac47058"),
	Challenge:       mustDecodeHash("12166ef1d5772aff8fa94a7248d8d3601f8a3eb2faf4e46820b0f91b4d36a508"),
	PubKey:          mustDecodePoCPublicKey("033e44859a79ea399c69c41c248b4b33384813fe7b34735dd7d117acc08ae04d42"),
	Proof: &poc.Proof{
		X:         mustDecodeString("07149d0c"),
		XPrime:    mustDecodeString("941ea70c"),
		BitLength: 28,
	},
	Signature: mustDecodePoCSignature("3045022100dbcbafc7b41d4622394c55527aa43f582be7c8e7106579d26cf4b1a443cfd7e902203104a8ada1e64db2df7e88d0dafe548299cd878adddaf41b323c4a855c926c88"),
	BanList:   make([]*pocec.PublicKey, 0),
}

// genesisBlock defines the genesis block of the block chain which serves as the
// public transaction ledger.
var genesisBlock = wire.MsgBlock{
	Header: genesisHeader,
	Proposals: wire.ProposalArea{
		PunishmentArea: make([]*wire.FaultPubKey, 0),
		OtherArea:      make([]*wire.NormalProposal, 0),
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}

var genesisHash = mustDecodeHash("7f56dee203d1798c2e34180ed8f763ea62e01758a3cfa91373999a1a1a7b53fb")

var genesisChainID = mustDecodeHash("3e5c9bbb72a812303dd01b99ffe7fba755a7272a3a42abf2e983fdc2c0ec34b8")

func hexToBigInt(str string) *big.Int {
	return new(big.Int).SetBytes(mustDecodeString(str))
}

func mustDecodeString(str string) []byte {
	buf, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}
	return buf
}

func mustDecodeHash(str string) wire.Hash {
	h, err := wire.NewHashFromStr(str)
	if err != nil {
		panic(err)
	}
	return *h
}

func mustDecodePoCPublicKey(str string) *pocec.PublicKey {
	pub, err := pocec.ParsePubKey(mustDecodeString(str), pocec.S256())
	if err != nil {
		panic(err)
	}
	return pub
}

func mustDecodePoCSignature(str string) *pocec.Signature {
	sig, err := pocec.ParseSignature(mustDecodeString(str), pocec.S256())
	if err != nil {
		panic(err)
	}
	return sig
}

type GenesisProof struct {
	x         string `json:"x"`
	y         string `json:"y"`
	BitLength int    `json:"bit_length"`
}

type GenesisSignature struct {
	r string `json:"r"`
	s string `json:"s"`
}

type GenesisAlloc struct {
	Value   uint64 `json:"value"`
	Address string `json:"address"`
}

// GenesisDoc defines the initial conditions for a sukhavati blockchain, in particular its validator set.
type GenesisDoc struct {
	Version    uint64           `json: "version"`
	InitHeight uint64           `json:"init_height"`
	Timestamp  uint64           `json:"timestamp"`
	Target     string           `json:"target"`
	Challenge  string           `json:"challenge"`
	PublicKey  string           `json:"public_key"`
	Proof      GenesisProof     `json:"proof"`
	Signature  GenesisSignature `json:"signature"`
	alloc      []GenesisAlloc   `json:"alloc"`
	AllocTxOut []*wire.TxOut
}

var ChainGenesisDoc GenesisDoc

const ChainGenesisDocHash = "73756b686176617469b34ea2f85159fa271423fcc27496b5e2"

func InitChainGenesisDoc() {
	ChainGenesisDoc = GenesisDoc{
		Version:    1,
		InitHeight: 1,
		Timestamp:  0,
		Target:     "44814ac47058",
		Challenge:  "",
		PublicKey:  "",
		Proof:      GenesisProof{},
		Signature:  GenesisSignature{},
		AllocTxOut: []*wire.TxOut{
			{
				Value:    0x20EF7AC3840A00, //Investor sk1qqrz45pn0x7nmqsl386yv8z77gpstkchzdzmfrppprazfk3xudrq3samq95a
				PkScript: mustDecodeString("002018ab40cde6f4f6087e27d118717bc80c176c5c4d16d2308423e893689b8d1823"),
			},
			{
				Value:    0xD9D02F39EBB00, //Ecology sk1qqkla5a9a05pcfavtwnz8m2r296yzvlak26n7ksd7ev3q24j22qj9sr78w7w
				PkScript: mustDecodeString("0020b7fb4e97afa0709eb16e988fb50d45d104cff6cad4fd6837d96440aac94a048b"),
			},
			{
				Value:    0x15F4FC8454A700, //Team sk1qq2sv82wygyega90g3amckmtanxgcyaesn9r40zcqekq0yy7nyl6yqmkju74
				PkScript: mustDecodeString("002054187538882651d2bd11eef16dafb332304ee61328eaf16019b01e427a64fe88"),
			},
			{
				Value:    0xF5EB0C13E4B00, //Foundation sk1qq049qd3e48d2g0l520ldk4akut9vl7f4sr6a8pmfs57gth2swevkqghmmny
				PkScript: mustDecodeString("00207d4a06c7353b5487fe8a7fdb6af6dc5959ff26b01eba70ed30a790bbaa0ecb2c"),
			},
		},
	}
}

// SaveAs is a utility method for saving GenesisDoc as a JSON file.
func (genDoc GenesisDoc) SaveAs(file string) error {
	genDocBytes, err := json.Marshal(genDoc)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, genDocBytes, 0644)
}

func (genDoc GenesisDoc) IsHashEqual(sha string) bool {
	var data bytes.Buffer
	for _, v := range genDoc.AllocTxOut {
		_, err := data.Write(v.PkScript)
		if err != nil {
			return false
		}
		var buf = make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(v.Value))
		_, err = data.Write(buf)
		if err != nil {
			return false
		}
	}
	hash := md5.New()
	hash.Write(data.Bytes())
	sum := hash.Sum([]byte("sukhavati"))
	//
	toString := hex.EncodeToString(sum)
	println("GenesisDocHash:" + toString)
	return bytes.Equal(sum, mustDecodeString(ChainGenesisDocHash))
}
