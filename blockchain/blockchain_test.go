package blockchain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newBlockChain() (*Blockchain, func(), error) {
	db, err := newTestChainDb()
	if err != nil {
		return nil, nil, err
	}

	teardown, err := mkTmpDir(dbPath)
	if err != nil {
		db.Close()
		return nil, nil, err
	}

	bc, err := newTestBlockchain(db, dbPath)
	if err != nil {
		db.Close()
		teardown()
		return nil, nil, err
	}
	return bc, func() {
		db.Close()
		teardown()
	}, nil
}

func TestNewBlockchain(t *testing.T) {
	bc, teardown, err := newBlockChain()
	assert.Nil(t, err)
	defer teardown()

	block, err := loadNthBlock(1)
	assert.Nil(t, err)

	assert.Equal(t, block.Hash(), bc.BestBlockHash())
	assert.Equal(t, block.Height(), bc.BestBlockHeight())
	t.Log("blockchain", bc.BestBlockHash(), bc.BestBlockHeight())
}

func TestBlockchain_ProcessBlock(t *testing.T) {
	// genesis already initialized
	bc, teardown, err := newBlockChain()
	assert.Nil(t, err)
	defer teardown()

	blocks, err := loadTopNBlock(10)
	assert.Nil(t, err)

	// logging.InitElk(logpath, config.DefaultElkFilename)

	// the 1st block is genesis
	for i := 1; i < 10; i++ {
		_, err = bc.processBlock(blocks[i], BFNone)
		if err != nil {
			t.Fatal("err in ProcessBlock", err)
		}
		assert.Equal(t, uint64(i), bc.BestBlockHeight())
	}
}

// This case simulates a possible scenario that some blocks arrive early.
func TestBlockchain_ProcessBlock2(t *testing.T) {
	// genesis already initialized
	bc, teardown, err := newBlockChain()
	assert.Nil(t, err)
	defer teardown()

	blocks, err := loadTopNBlock(10)
	assert.Nil(t, err)

	// logging.InitElk(logpath, config.DefaultElkFilename)

	// block(height=7) arrives
	_, err = bc.processBlock(blocks[7], BFNone)
	if err != nil {
		t.Fatal("err in ProcessBlock", err)
	}
	assert.Equal(t, uint64(0), bc.BestBlockHeight())

	// block(height=5) arrives
	_, err = bc.processBlock(blocks[5], BFNone)
	if err != nil {
		t.Fatal("err in ProcessBlock", err)
	}
	assert.Equal(t, uint64(0), bc.BestBlockHeight())

	// blocks 1~3 arrive in order
	for i := 1; i < 4; i++ {
		_, err = bc.processBlock(blocks[i], BFNone)
		if err != nil {
			t.Fatal("err in ProcessBlock", err)
		}
		assert.Equal(t, uint64(i), bc.BestBlockHeight())
	}

	// block 4 arrives
	_, err = bc.processBlock(blocks[4], BFNone)
	if err != nil {
		t.Fatal("err in ProcessBlock", err)
	}
	// block 5 already arrived
	assert.Equal(t, uint64(5), bc.BestBlockHeight())

	// block 6 arrives
	_, err = bc.processBlock(blocks[6], BFNone)
	if err != nil {
		t.Fatal("err in ProcessBlock", err)
	}
	// block 7 already arrived
	assert.Equal(t, uint64(7), bc.BestBlockHeight())
}
