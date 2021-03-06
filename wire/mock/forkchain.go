package mock

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/Sukhavati-Labs/go-miner/consensus"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

type ForkPoint byte

const (
	FPStakingBegin ForkPoint = 1 << iota
	FPStakingEnd
	FPStakingRewardBegin
	FPBindingBegin
	FPStandardOnce
)

type ForkOptions struct {
	maxForks int // number of forks at each point

	// mine D blocks on each fork since fork point
	maxBlockOnFork int

	// Chain forks at height FP + FPOffset
	FP ForkPoint
	// FPOffset int8
}

var (
	defaultForkOpt = &ForkOptions{
		maxForks:       1,
		maxBlockOnFork: 3,
		FP:             FPStakingBegin,
		// FPOffset:     -1,
	}
)

type randEmitter struct {
	mu  sync.Mutex
	his map[int]struct{}
}

func (e *randEmitter) Get() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	for {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		d := r.Intn(math.MaxInt64)
		if _, ok := e.his[d]; ok {
			continue
		}
		e.his[d] = struct{}{}
		return d
	}
}

func doFork(chain *Chain) *Chain {

	fork := &Chain{
		opt:    chain.opt,
		blocks: make([]*wire.MsgBlock, len(chain.blocks), len(chain.blocks)),
		gUtxoMgr: &gUtxoMgr{
			pocAddr2utxos: make(map[string][]*bindingUtxo),
			op2gutxo:      make(map[wire.OutPoint]*bindingUtxo),
		},
		lUtxoMgr: &lUtxoMgr{
			addr2utxos: make(map[string][]*utxo),
			op2lutxo:   make(map[wire.OutPoint]*utxo),
		},
		utxos:       make(map[uint64][]*utxo),
		txIndex:     make(map[wire.Hash]*txLoc),
		owner2utxos: make(map[string][]*utxo),

		stats:           make([]*BlockTxStat, 0, len(chain.stats)),
		forkStartHeight: int64(chain.constructHeight),
		constructHeight: chain.constructHeight,
		randEmitter:     chain.randEmitter,
	}

	fork.lUtxoMgr.c = fork
	fork.gUtxoMgr.c = fork
	if fork.opt.ForkOpt == nil {
		fork.opt.ForkOpt = defaultForkOpt
	}

	var err error
	for i, block := range chain.blocks {
		fork.blocks[i], err = copyBlock(block)
		if err != nil {
			panic(err)
		}
	}

	for h, l := range chain.txIndex {
		fork.txIndex[h] = l
	}
	for _, s := range chain.stats {
		fork.stats = append(fork.stats, s)
	}

	for owner, utxos := range chain.owner2utxos {
		fork.owner2utxos[owner] = make([]*utxo, 0, len(utxos))
		for _, u := range utxos {
			nu := &utxo{
				prePoint:     u.prePoint,
				blockIndex:   u.blockIndex,
				txIndex:      u.txIndex,
				redeemScript: u.redeemScript,
				privateKey:   u.privateKey,
				value:        u.value,
				spent:        u.spent,
				blockHeight:  u.blockHeight,
				maturity:     u.maturity,
				pocAddr:      u.pocAddr,
				isStaking:    u.isStaking,
			}
			fork.owner2utxos[owner] = append(fork.owner2utxos[owner], nu)
			fork.utxos[nu.blockHeight] = append(fork.utxos[nu.blockHeight], nu)

			if nu.pocAddr != nil {
				if nu.isStaking {
					panic("impossible")
				}
				miner := nu.pocAddr.EncodeAddress()
				gSlice := fork.gUtxoMgr.pocAddr2utxos[miner]
				gUtxo := &bindingUtxo{
					utxo:           nu,
					bindingAddress: nu.pocAddr,
				}
				gSlice = append(gSlice, gUtxo)
				fork.gUtxoMgr.pocAddr2utxos[miner] = gSlice
				fork.gUtxoMgr.op2gutxo[*nu.prePoint] = gUtxo
				continue
			}
			if nu.isStaking {
				fork.lUtxoMgr.addr2utxos[owner] = append(fork.lUtxoMgr.addr2utxos[owner], nu)
				fork.lUtxoMgr.op2lutxo[*nu.prePoint] = nu
			}
		}
	}
	return fork
}

func (c *Chain) autoMockForkChain() error {
	totalHeight, txPerBlock := c.opt.TotalHeight, c.opt.TxPerBlock
	c.blocks = make([]*wire.MsgBlock, c.opt.TotalHeight, c.opt.TotalHeight)

	// fill in basic blocks
	for i := int64(0); i < totalHeight; i++ {
		block, err := copyBlock(basicBlocks[i])
		if err != nil {
			return err
		}
		c.blocks[i] = block
	}

	ensureNumberKeys(txPerBlock)

	forks := make(map[*Chain]int) // value is fork index
	flagStdOnceExecuted := false
	c.allBlocks = append(c.allBlocks, c.blocks[0])
	lastForkHeight := 0
	stakingRewardForkPoint := make(map[int]struct{})
	for i := 1; i < int(totalHeight); i++ {
		if i < challengeInterval+1 {
			err := c.retrieveCoinbase(c.blocks[i], uint64(i))
			if err != nil {
				return err
			}
			c.allBlocks = append(c.allBlocks, c.blocks[i])
			c.constructHeight = uint64(i)
			continue
		}

		if i%20 == 0 {
			c.clearUtxos()
		}

		forkTpl := doFork(c)

		block, err := c.constructBlock(c.blocks[i], uint64(i))
		if err != nil {
			return err
		}
		c.blocks[i] = block
		c.allBlocks = append(c.allBlocks, block)
		// fmt.Printf("best chain height %d has tx %d\n", block.Header.Height, len(block.Transactions))

		if i <= lastForkHeight+c.opt.ForkOpt.maxBlockOnFork || i >= int(c.opt.TotalHeight)-c.opt.ForkOpt.maxBlockOnFork {
			continue
		}
		// mock forks
		switch c.opt.ForkOpt.FP {
		case FPStakingBegin:
			if c.stats[len(c.stats)-1].stat[TxStaking] > 0 {
				fmt.Println("")
				for len(forks) < c.opt.ForkOpt.maxForks {
					fk := doFork(forkTpl)
					fmt.Printf("fork [%d-%d] add FPStakingBegin, next height: %d\n", fk.ForkStartHeight(), len(forks), i)
					forks[fk] = len(forks)
				}
			}
		case FPStakingEnd:
			for _, utxo := range c.lUtxoMgr.op2lutxo {
				if !utxo.spent && int(utxo.blockHeight+utxo.maturity-1) == i {
					for len(forks) < c.opt.ForkOpt.maxForks {
						fk := doFork(forkTpl)
						fmt.Printf("fork [%d-%d] add (FPStakingEnd) \n			i: %d, blockHeight: %d, mat: %d\n",
							fk.ForkStartHeight(), len(forks), i, utxo.blockHeight, utxo.maturity)
						forks[fk] = len(forks)
					}
					break
				}
			}
		case FPBindingBegin:
			if c.stats[len(c.stats)-1].stat[TxBinding] > 0 {
				fmt.Println("")
				for len(forks) < c.opt.ForkOpt.maxForks {
					fk := doFork(forkTpl)
					fmt.Printf("fork [%d-%d] add FPBindingBegin, next height: %d\n", fk.ForkStartHeight(), len(forks), i)
					forks[fk] = len(forks)
				}
			}
		case FPStakingRewardBegin:
			if c.stats[len(c.stats)-1].stat[TxStaking] > 0 {
				stakingRewardForkPoint[i+int(consensus.StakingTxRewardStart)-1] = struct{}{}
			}
			if _, ok := stakingRewardForkPoint[i-1]; ok {
				for len(forks) < c.opt.ForkOpt.maxForks {
					fk := doFork(forkTpl)
					fmt.Printf("fork [%d-%d] add FPStakingRewardBegin, start height: %d\n", fk.ForkStartHeight(), len(forks), i)
					forks[fk] = len(forks)
				}
				delete(stakingRewardForkPoint, i-1)
			}
		case FPStandardOnce:
			if i > 25 && c.stats[len(c.stats)-1].stat[TxNormal] > 0 &&
				len(forks) < c.opt.ForkOpt.maxForks && !flagStdOnceExecuted {
				fk := doFork(forkTpl)
				fmt.Printf("fork [%d-%d] add FPStandardOnce, next height: %d\n", fk.ForkStartHeight(), len(forks), i)
				forks[fk] = len(forks)
				flagStdOnceExecuted = true
			}
		default:
			panic("unknown fork options")
		}

		for fork, idx := range forks {
			fmt.Printf("fork [%d-%d] mock from height : %d, initial allBlocks: %d\n", fork.ForkStartHeight(), idx, i, len(c.allBlocks))

			for executed := 0; executed < c.opt.ForkOpt.maxBlockOnFork; executed++ {
				k := i + executed

				block, err := fork.constructBlock(fork.blocks[k], uint64(k))
				if err != nil {
					panic(fmt.Errorf("fork [%d-%d] exec error : %v, times: %d\n", fork.ForkStartHeight(), idx, err, k))
				}

				if executed == 0 {
					fbSha := block.BlockHash()
					mbSha := c.blocks[i].BlockHash()
					if bytes.Equal(fbSha[:], mbSha[:]) {
						panic(fmt.Errorf("	fork failed for dup block: %d %s %s", i, fbSha.String(), c.blocks[i].BlockHash().String()))
					}
				}

				fork.blocks[k] = block
				c.allBlocks = append(c.allBlocks, block)
				fmt.Printf("		    > height : %d, allBlocks: %d, header height: %d, txs: %d\n", k, len(c.allBlocks), block.Header.Height, len(block.Transactions))
				if k > lastForkHeight {
					lastForkHeight = k
				}
			}

			fmt.Printf("fork [%d-%d] delete, final allBlocks : %d\n", fork.ForkStartHeight(), idx, len(c.allBlocks))
			delete(forks, fork)
		}

		if len(forks) != 0 {
			panic("not empty")
		}

	}
	return nil
}
