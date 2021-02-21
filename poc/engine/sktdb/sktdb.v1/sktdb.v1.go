package sktdb_v1

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Sukhavati-Labs/go-miner/poc"
	"github.com/Sukhavati-Labs/go-miner/poc/engine/sktdb"
	"github.com/Sukhavati-Labs/go-miner/poc/pocutil"
	"github.com/Sukhavati-Labs/go-miner/pocec"
)

type MapType uint8

const (
	// TODO Compatible with multiple db formats
	TypeSktDBV1             = "massdb.v1"
	MapTypeHashMapA MapType = iota
	MapTypeHashMapB
)

type SktDBV1 struct {
	HashMapA   *HashMapA
	HashMapB   *HashMapB
	filePathA  string
	filePathB  string
	bl         int
	pubKey     *pocec.PublicKey
	pubKeyHash pocutil.Hash
	plotting   int32 // atomic
	stopPlotCh chan struct{}
	wg         sync.WaitGroup
}

func (mdb *SktDBV1) Type() string {
	return TypeSktDBV1
}

func (mdb *SktDBV1) Close() error {
	<-mdb.StopPlot()

	if mdb.HashMapA != nil {
		mdb.HashMapA.Close()
	}
	if mdb.HashMapB != nil {
		mdb.HashMapB.Close()
	}
	return nil
}

// Plot is concurrent safe, it starts the plotting work,
// running actual plot func as a thread
func (mdb *SktDBV1) Plot() chan error {
	result := make(chan error, 1)

	if !atomic.CompareAndSwapInt32(&mdb.plotting, 0, 1) {
		result <- ErrAlreadyPlotting
		return result
	}

	if mdb.HashMapA == nil {
		result <- nil
		return result
	}

	mdb.stopPlotCh = make(chan struct{})
	mdb.wg.Add(1)
	go mdb.executePlot(result)

	return result
}

// StopPlot stops plot process
func (mdb *SktDBV1) StopPlot() chan error {
	result := make(chan error, 1)

	if atomic.LoadInt32(&mdb.plotting) == 0 {
		result <- nil
		return result
	}

	go func() {
		close(mdb.stopPlotCh)
		mdb.wg.Wait()
		result <- nil
	}()
	return result
}

func (mdb *SktDBV1) Ready() bool {
	plotted, _ := mdb.HashMapB.Progress()
	return plotted
}

func (mdb *SktDBV1) BitLength() int {
	return mdb.bl
}

func (mdb *SktDBV1) PubKeyHash() pocutil.Hash {
	return mdb.pubKeyHash
}

func (mdb *SktDBV1) PubKey() *pocec.PublicKey {
	return mdb.pubKey
}

func (mdb *SktDBV1) Get(z pocutil.PoCValue) (x, xp pocutil.PoCValue, err error) {
	var bl = mdb.bl
	xb, xpb, err := mdb.HashMapB.Get(z)
	if err != nil {
		return 0, 0, err
	}
	return pocutil.Bytes2PoCValue(xb, bl), pocutil.Bytes2PoCValue(xpb, bl), nil
}

func (mdb *SktDBV1) GetProof(challenge pocutil.Hash) (*poc.Proof, error) {
	var bl = mdb.bl
	x, xp, err := mdb.HashMapB.Get(pocutil.CutHash(challenge, bl))
	if err != nil {
		return nil, err
	}
	proof := &poc.Proof{
		X:         x,
		XPrime:    xp,
		BitLength: bl,
	}
	err = poc.VerifyProof(proof, mdb.pubKeyHash, challenge)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

func (mdb *SktDBV1) Progress() (prePlotted, plotted bool, progress float64) {
	if mdb.HashMapA != nil {
		prePlotted, progA := mdb.HashMapA.Progress()
		plotted, progB := mdb.HashMapB.Progress()

		totalRecord := mdb.HashMapA.volume + mdb.HashMapB.volume
		currentRecord := progA + progB*2
		progress = float64(currentRecord*100) / float64(totalRecord)

		return prePlotted, plotted, progress
	} else {
		return true, true, 100
	}
}

func (mdb *SktDBV1) Delete() chan error {
	result := make(chan error, 1)

	var sendResult = func(err error) {
		result <- err
	}

	if atomic.LoadInt32(&mdb.plotting) != 0 {
		sendResult(ErrAlreadyPlotting)
		return result
	}

	if mdb.HashMapA != nil {
		mdb.HashMapA.Close()
	}
	mdb.HashMapB.Close()

	go func() {
		var errA, errB error
		if mdb.HashMapA != nil {
			errA = os.Remove(mdb.filePathA)
		}
		errB = os.Remove(mdb.filePathB)

		if errA == nil && errB == nil {
			sendResult(nil)
		}
		if errA != nil && errB == nil {
			sendResult(errors.New("A " + errA.Error()))
		}
		if errA == nil && errB != nil {
			sendResult(errors.New("B " + errB.Error()))
		}
		if errA != nil && errB != nil {
			sendResult(errors.New(strings.Join([]string{"A", errA.Error(), "B", errB.Error()}, " ")))
		}
	}()
	return result
}

func OpenDB(args ...interface{}) (sktdb.SktDB, error) {
	dbPath, ordinal, pubKey, bitLength, err := parseArgs(args...)
	if err != nil {
		return nil, err
	}

	pathA, pathB := getPath(dbPath, int(ordinal), pubKey, bitLength)
	hmBi, err := LoadHashMap(pathB)
	if err != nil {
		return nil, err
	}
	hmB, ok := hmBi.(*HashMapB)
	if !ok {
		return nil, ErrDBWrongType
	}

	var hmA *HashMapA
	hmA = nil
	if plotted, _ := hmB.Progress(); !plotted {
		hmAi, err := LoadHashMap(pathA)
		if err != nil {
			return nil, err
		}
		hmA, ok = hmAi.(*HashMapA)
		if !ok {
			return nil, ErrDBWrongType
		}
	}

	return &SktDBV1{
		HashMapA:   hmA,
		HashMapB:   hmB,
		filePathA:  pathA,
		filePathB:  pathB,
		bl:         bitLength,
		pubKey:     pubKey,
		pubKeyHash: pocutil.PubKeyHash(pubKey),
	}, nil
}

func CreateDB(args ...interface{}) (sktdb.SktDB, error) {
	dbPath, ordinal, pubKey, bitLength, err := parseArgs(args...)
	if err != nil {
		return nil, err
	}

	pathA, pathB := getPath(dbPath, int(ordinal), pubKey, bitLength)
	if err := CreateHashMap(pathA, MapTypeHashMapA, bitLength, pubKey); err != nil && err != sktdb.ErrDBAlreadyExists {
		return nil, err
	}
	if err := CreateHashMap(pathB, MapTypeHashMapB, bitLength, pubKey); err != nil && err != sktdb.ErrDBAlreadyExists {
		return nil, err
	}
	hmAi, err := LoadHashMap(pathA)
	if err != nil {
		return nil, err
	}
	hmBi, err := LoadHashMap(pathB)
	if err != nil {
		return nil, err
	}
	hmA, ok := hmAi.(*HashMapA)
	if !ok {
		return nil, ErrDBWrongType
	}
	hmB, ok := hmBi.(*HashMapB)
	if !ok {
		return nil, ErrDBWrongType
	}

	return &SktDBV1{
		HashMapA:   hmA,
		HashMapB:   hmB,
		filePathA:  pathA,
		filePathB:  pathB,
		bl:         bitLength,
		pubKey:     pubKey,
		pubKeyHash: pocutil.PubKeyHash(pubKey),
	}, nil
}

func getPath(rootPath string, ordinal int, pubKey *pocec.PublicKey, bitLength int) (pathA, pathB string) {
	pubKeyString := hex.EncodeToString(pubKey.SerializeCompressed())
	pathA = strings.Join([]string{strconv.Itoa(ordinal), pubKeyString, strconv.Itoa(bitLength), "a"}, "_") + ".sktdb"
	pathB = strings.Join([]string{strconv.Itoa(ordinal), pubKeyString, strconv.Itoa(bitLength)}, "_") + ".sktdb"
	return filepath.Join(rootPath, pathA), filepath.Join(rootPath, pathB)
}

func parseArgs(args ...interface{}) (string, int64, *pocec.PublicKey, int, error) {
	if len(args) != 4 {
		return "", 0, nil, 0, sktdb.ErrInvalidDBArgs
	}
	dbPath, ok := args[0].(string)
	if !ok {
		return "", 0, nil, 0, sktdb.ErrInvalidDBArgs
	}
	ordinal, ok := args[1].(int64)
	if !ok {
		return "", 0, nil, 0, sktdb.ErrInvalidDBArgs
	}
	pubKey, ok := args[2].(*pocec.PublicKey)
	if !ok {
		return "", 0, nil, 0, sktdb.ErrInvalidDBArgs
	}
	bitLength, ok := args[3].(int)
	if !ok {
		return "", 0, nil, 0, sktdb.ErrInvalidDBArgs
	}

	return dbPath, ordinal, pubKey, bitLength, nil
}

func init() {
	sktdb.AddDBBackend(sktdb.DBBackend{
		Typ:      TypeSktDBV1,
		OpenDB:   OpenDB,
		CreateDB: CreateDB,
	})
}

func NewSktDBV1ForTest(filePath string) (*SktDBV1, error) {
	hmBi, err := LoadHashMap(filePath)
	if err != nil {
		return nil, err
	}
	hmB, ok := hmBi.(*HashMapB)
	if !ok {
		return nil, ErrDBWrongType
	}

	return &SktDBV1{
		HashMapB:   hmB,
		filePathB:  filePath,
		bl:         hmB.bl,
		pubKeyHash: hmB.pkHash,
		pubKey:     hmB.pk,
	}, nil
}

func NewSktDBV1MapA(filePath string) (*HashMapA, error) {
	hmAi, err := LoadHashMap(filePath)
	if err != nil {
		return nil, err
	}
	hmA, ok := hmAi.(*HashMapA)
	if !ok {
		return nil, ErrDBWrongType
	}

	return hmA, nil
}

func (hm *HashMapA) BitLength() int {
	return hm.bl
}

func (hm *HashMapA) PubKey() *pocec.PublicKey {
	return hm.pk
}

func (hm *HashMapA) PubKeyHash() pocutil.Hash {
	return hm.pkHash
}
