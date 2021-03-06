package ldb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/errors"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

var (
	ErrUpgradeHeight     = errors.New("upgrade error: height")
	ErrUpgradeBlockHash  = errors.New("upgrade error: block hash")
	ErrUpgradeFileNumber = errors.New("upgrade error: file number")
)

func (db *ChainDb) Upgrade_1_1_0() error {
	// step 1
	// move blocks to disk
	err := moveBlockToDisk(db, "[1/3]")
	if err != nil {
		return err
	}

	// step 2
	// remove empty BANHGT
	err = removeEmptyBan(db, "[2/3]")
	if err != nil {
		return err
	}
	// step 3
	// build STL/HTS index
	return buildTxIndex(db, "[3/3]")
}

func moveBlockToDisk(db *ChainDb, progStage string) error {

	// get progress
	fbs, err := db.getAllBlockFileMeta()
	if err != nil {
		return err
	}

	var end uint64
	files := make(map[uint32]struct{})
	for _, fb := range fbs {
		fileNo := binary.LittleEndian.Uint32(fb[0:4])
		files[fileNo] = struct{}{}
		if int(fileNo) == len(fbs)-1 {
			end = binary.LittleEndian.Uint64(fb[24:32]) + 1 // highest block + 1
			continue
		}
		if int(fileNo) > len(fbs)-1 {
			logging.CPrint(logging.ERROR, "unexpected error", logging.LogFormat{
				"fileNo": fileNo,
				"total":  len(fbs),
			})
			return ErrUpgradeFileNumber
		}
	}
	if len(files) != len(fbs) || len(fbs) == 0 {
		logging.CPrint(logging.ERROR, "unexpected error", logging.LogFormat{
			"files":  files,
			"expect": len(fbs),
		})
		return ErrUpgradeFileNumber
	}
	if db.dbStorageMeta.currentHeight == UnknownHeight {
		return ErrUpgradeHeight
	}

	// find the start height
	start := uint64(0)
	if end > 100 {
		start = end - 100
	}
	for ; start < end; start++ {
		key := makeBlockHeightKey(start)
		value, err := db.localStorage.Get(key)
		if err != nil {
			logging.CPrint(logging.ERROR, "locate error", logging.LogFormat{
				"height": start,
				"err":    err,
			})
			return err
		}
		if len(value) != 52 {
			break
		}
	}

	logging.CPrint(logging.INFO, fmt.Sprintf("%s mv blocks to disk", progStage), logging.LogFormat{
		"from_height": start,
		"to_height":   db.dbStorageMeta.currentHeight,
		"latest_file": len(fbs) - 1,
	})

	prog := 0
	for cur := start; cur <= db.dbStorageMeta.currentHeight; cur++ {
		batch := db.localStorage.NewBatch()

		key := makeBlockHeightKey(cur)
		value, err := db.localStorage.Get(key)
		if err != nil {
			logging.CPrint(logging.ERROR, "get BLKHGT value error", logging.LogFormat{
				"height": cur,
				"err":    err,
			})
			return err
		}
		if len(value) == 52 {
			continue
		}
		// block sha
		var blockSha wire.Hash
		copy(blockSha[:], value[:32])
		rawBlock := value[32:]

		block, err := chainutil.NewBlockFromBytes(rawBlock, wire.DB)
		if err != nil {
			return err
		}

		blockFile, offset, err := db.blockFileKeeper.SaveRawBlockToDisk(rawBlock, cur, block.MsgBlock().Header.Timestamp.Unix())
		if err != nil {
			logging.CPrint(logging.ERROR, "SaveRawBlockToDisk error", logging.LogFormat{
				"block":  blockSha,
				"height": cur,
				"err":    err,
			})
			return err
		}

		// update block index
		blockHeightValue := make([]byte, 52)
		copy(blockHeightValue[0:], blockSha[:])
		binary.LittleEndian.PutUint32(blockHeightValue[32:], blockFile.Number())
		binary.LittleEndian.PutUint64(blockHeightValue[32+4:], uint64(offset))
		binary.LittleEndian.PutUint64(blockHeightValue[32+12:], uint64(len(rawBlock)))
		err = batch.Put(key, blockHeightValue)
		if err != nil {
			logging.CPrint(logging.ERROR, "update BLKHGT error", logging.LogFormat{
				"block":  blockSha,
				"height": cur,
				"err":    err,
			})
			return err
		}

		err = putLatestBlockFileMeta(batch, blockFile.Bytes())
		if err != nil {
			logging.CPrint(logging.ERROR, "putLatestBlockFileMeta error", logging.LogFormat{
				"block":  blockSha,
				"height": cur,
				"err":    err,
			})
			return err
		}

		// commit db
		err = db.localStorage.Write(batch)
		if err != nil {
			logging.CPrint(logging.ERROR, "write db error", logging.LogFormat{
				"block":  blockSha,
				"height": cur,
				"err":    err,
			})
			return err
		}
		db.blockFileKeeper.CommitRecentChange()

		// print progress
		newProg := (cur - start) * 100 / (db.dbStorageMeta.currentHeight - start)
		if int(newProg) >= prog+5 {
			prog = int(newProg)
			logging.CPrint(logging.INFO, fmt.Sprintf("%s %d%%", progStage, prog), logging.LogFormat{})
		}
	}
	logging.CPrint(logging.INFO, fmt.Sprintf("%s done", progStage), logging.LogFormat{})
	return nil
}

func removeEmptyBan(db *ChainDb, progStage string) error {
	logging.CPrint(logging.INFO, fmt.Sprintf("%s rm empty BANHGT", progStage), logging.LogFormat{})
	removed := 0
	iter := db.localStorage.NewIterator(storage.BytesPrefix(faultPubKeyHeightShaPrefix))
	defer iter.Release()
	for iter.Next() {
		value := iter.Value()
		count := binary.LittleEndian.Uint16(value[:2])
		if count > 0 {
			continue
		}
		err := db.localStorage.Delete(iter.Key())
		if err != nil {
			logging.CPrint(logging.ERROR, "remove ban error", logging.LogFormat{"err": err})
			return err
		}
		removed++
	}
	logging.CPrint(logging.INFO, fmt.Sprintf("%s done", progStage), logging.LogFormat{"removed": removed})
	return nil
}

func removeTxIndex(db *ChainDb, progStage string) error {
	batch := db.localStorage.NewBatch()
	defer batch.Release()

	// remove HTS
	logging.CPrint(logging.INFO, fmt.Sprintf("%s remove HTS start", progStage), logging.LogFormat{})
	countHts := 0
	iterHts := db.localStorage.NewIterator(storage.BytesPrefix(shIndexPrefix))
	defer iterHts.Release()
	for iterHts.Next() {
		err := batch.Delete(iterHts.Key())
		if err != nil {
			return err
		}
		countHts++
		if countHts > 0 && countHts%5000 == 0 {
			if err := db.localStorage.Write(batch); err != nil {
				logging.CPrint(logging.ERROR, fmt.Sprintf("%s remove HTS error", progStage), logging.LogFormat{
					"err":   err,
					"batch": countHts / 5000,
				})
				return err
			}
			batch.Reset()
		}
	}
	if err := db.localStorage.Write(batch); err != nil {
		logging.CPrint(logging.ERROR, fmt.Sprintf("%s remove HTS error", progStage), logging.LogFormat{
			"err": err,
		})
		return err
	}
	batch.Reset()
	logging.CPrint(logging.INFO, fmt.Sprintf("%s remove HTS done", progStage), logging.LogFormat{"removed": countHts})

	// remove STL
	logging.CPrint(logging.INFO, fmt.Sprintf("%s remove STL start", progStage), logging.LogFormat{})
	countStl := 0
	iterStl := db.localStorage.NewIterator(storage.BytesPrefix(txIndexPrefix))
	defer iterStl.Release()
	for iterStl.Next() {
		err := batch.Delete(iterStl.Key())
		if err != nil {
			return err
		}
		countStl++
		if countStl > 0 && countStl%5000 == 0 {
			if err := db.localStorage.Write(batch); err != nil {
				logging.CPrint(logging.ERROR, fmt.Sprintf("%s remove STL error", progStage), logging.LogFormat{
					"err":   err,
					"batch": countStl / 5000,
				})
				return err
			}
			batch.Reset()
		}
	}
	if err := db.localStorage.Write(batch); err != nil {
		logging.CPrint(logging.ERROR, fmt.Sprintf("%s remove STL error", progStage), logging.LogFormat{
			"err": err,
		})
		return err
	}
	logging.CPrint(logging.INFO, fmt.Sprintf("%s remove STL done", progStage), logging.LogFormat{"removed": countStl})
	return nil
}

func writeTxIndex(db *ChainDb, m map[[43]byte][]byte) error {
	for k, v := range m {
		err := db.localStorage.Put(k[:], v)
		if err != nil {
			return err
		}
	}
	return nil
}

func buildTxIndex(db *ChainDb, progStage string) error {
	err := removeTxIndex(db, progStage)
	if err != nil {
		logging.CPrint(logging.ERROR, "remove tx index error", logging.LogFormat{"err": err})
		return err
	}

	logging.CPrint(logging.INFO, fmt.Sprintf("%s build HTS/STL start", progStage), logging.LogFormat{})

	type TxInfo struct {
		txhash       wire.Hash
		height       uint64
		loc          wire.TxLoc
		spentBuf     []byte
		isFullySpent bool
	}

	_, bestHeight, _ := db.NewestSha()
	if bestHeight == UnknownHeight {
		return ErrUpgradeHeight
	}

	spentOutpoints := make(map[wire.OutPoint]wire.Hash)
	currentHtsMap := make(map[[43]byte][]byte)
	currentStlMap := make(map[[43]byte][]byte)
	prog := 0
	for height := uint64(0); height <= bestHeight; height++ {
		if height > 0 {
			if height%htsBoundaryDistance == 0 {
				if err := writeTxIndex(db, currentHtsMap); err != nil {
					return err
				}
				currentHtsMap = make(map[[43]byte][]byte)
			}
			if height%stlBoundaryDistance == 0 {
				if err := writeTxIndex(db, currentStlMap); err != nil {
					return err
				}
				currentStlMap = make(map[[43]byte][]byte)
			}
		}
		sha, buf, err := db.getBlockByHeight(height)
		if err != nil {
			return err
		}
		block, err := chainutil.NewBlockFromBytes(buf, wire.DB)
		if err != nil {
			return err
		}
		if !bytes.Equal(sha[:], block.Hash()[:]) {
			return ErrUpgradeBlockHash
		}

		// parse block transactions
		blockRelatedScriptHash := make(map[wire.Hash]map[wire.TxLoc]bool)
		for txIdx, tx := range block.MsgBlock().Transactions {
			var txInfo *TxInfo
			// fully spent
			list, err := db.getTxFullySpent(tx.TxHash().Ptr())
			if err != nil && err != storage.ErrNotFound {
				logging.CPrint(logging.ERROR, "getTxFullySpent error", logging.LogFormat{
					"err":    err,
					"height": height,
					"tx":     tx.TxHash(),
				})
				return err
			}
			for _, sptx := range list {
				if sptx.blockHeight == height {
					if sptx.numTxOut%8 != 0 || sptx.numTxOut < len(tx.TxOut) {
						return errors.New("incorrect TxOut number")
					}
					txInfo = &TxInfo{
						height: height,
						loc: wire.TxLoc{
							TxStart: sptx.txOffset,
							TxLen:   sptx.txLen,
						},
						isFullySpent: true,
					}
					break
				}
			}

			if txInfo == nil {
				// non fully spent
				txInfo = &TxInfo{}
				txInfo.height, txInfo.loc.TxStart, txInfo.loc.TxLen, txInfo.spentBuf, err = db.getTxData(tx.TxHash().Ptr())
				if err != nil ||
					txInfo.height != height ||
					len(txInfo.spentBuf)*8 < len(tx.TxOut) {
					logging.CPrint(logging.ERROR, "getTxData error",
						logging.LogFormat{
							"err":      err,
							"height":   height,
							"tx":       tx.TxHash(),
							"txHeight": txInfo.height,
						})
					return errors.New("getTxData error")
				}
			}
			txInfo.txhash = tx.TxHash()

			if txIdx != 0 { // skip coinbase
				for vin, txIn := range tx.TxIn {
					if scriptHash, ok := spentOutpoints[txIn.PreviousOutPoint]; ok {
						delete(spentOutpoints, txIn.PreviousOutPoint)
						if _, ok := blockRelatedScriptHash[scriptHash]; !ok {
							blockRelatedScriptHash[scriptHash] = make(map[wire.TxLoc]bool)
						}
						blockRelatedScriptHash[scriptHash][txInfo.loc] = true
					} else {
						logging.CPrint(logging.FATAL, "unexpected prev outpoint",
							logging.LogFormat{
								"height": height,
								"tx":     txInfo.txhash,
								"txidx":  txIdx,
								"vin":    vin,
							})
					}
				}
			}

			for vout, txOut := range tx.TxOut {
				class, pops := txscript.GetScriptInfo(txOut.PkScript)
				_, scriptHash, _, err := txscript.GetParsedOpcode(pops, class)
				if err != nil {
					return err
				}
				if _, ok := blockRelatedScriptHash[scriptHash]; !ok {
					blockRelatedScriptHash[scriptHash] = make(map[wire.TxLoc]bool)
				}
				blockRelatedScriptHash[scriptHash][txInfo.loc] = true

				spent := false
				if txInfo.isFullySpent {
					spent = true
				} else {
					byteidx := vout / 8
					byteoff := uint(vout % 8)
					spent = txInfo.spentBuf[byteidx]&(byte(1)<<byteoff) != 0
				}

				if spent {
					op := wire.OutPoint{Hash: txInfo.txhash, Index: uint32(vout)}
					spentOutpoints[op] = scriptHash
				}
			}
		} // range block transaction

		// construct HTS/STL record
		for scripthash, m := range blockRelatedScriptHash {
			slice := make([]wire.TxLoc, 0, len(m))
			for txloc := range m {
				slice = append(slice, txloc)
			}
			sort.Slice(slice, func(i, j int) bool {
				return slice[i].TxStart < slice[j].TxStart
			})

			// hts
			var htsKey [43]byte
			htsKeySlice, htsLowBound := encodeHTSKey(height, scripthash)
			copy(htsKey[:], htsKeySlice)
			htsBitmap := uint32(0)
			htsValue, ok := currentHtsMap[htsKey]
			if ok {
				htsBitmap = binary.LittleEndian.Uint32(htsValue)
			} else {
				htsValue = make([]byte, 4)
			}
			htsBitmap |= 0x01 << (height - htsLowBound)
			binary.LittleEndian.PutUint32(htsValue, htsBitmap)
			currentHtsMap[htsKey] = htsValue

			// stl
			var stlKey [43]byte
			stlKeySlice, stlLowBound := encodeSTLKey(height, scripthash)
			copy(stlKey[:], stlKeySlice)
			stlBitmap := uint32(0)
			stlValue, ok := currentStlMap[stlKey]
			if ok {
				stlBitmap = binary.LittleEndian.Uint32(stlValue[0:4])
			} else {
				stlValue = make([]byte, 4)
			}
			stlBitmap |= 0x01 << (height - stlLowBound)
			binary.LittleEndian.PutUint32(stlValue[0:4], stlBitmap) // set bitmap
			suffix := make([]byte, 3+8*len(slice))
			suffix[0] = byte(height - stlLowBound)                         // set index
			binary.LittleEndian.PutUint16(suffix[1:3], uint16(len(slice))) // set count
			start := 3
			for _, txloc := range slice { // set txloc
				binary.LittleEndian.PutUint32(suffix[start:start+4], uint32(txloc.TxStart))
				binary.LittleEndian.PutUint32(suffix[start+4:start+8], uint32(txloc.TxLen))
				start += 8
			}
			currentStlMap[stlKey] = append(stlValue, suffix...)
		}

		if int(height*100/bestHeight) >= prog+5 {
			prog = int(height * 100 / bestHeight)
			logging.CPrint(logging.INFO, fmt.Sprintf("%s %d%%", progStage, prog), logging.LogFormat{})
		}
	}

	err = writeTxIndex(db, currentHtsMap)
	if err != nil {
		return err
	}
	return writeTxIndex(db, currentStlMap)
}
