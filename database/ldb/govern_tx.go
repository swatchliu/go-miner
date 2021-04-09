package ldb

import (
	"encoding/binary"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

//

var (
	recordGovernTx    = []byte("TXG")
	recordGovernTxLen = len(recordGovernTx)
)

const (
	//
	//  +---------+--------+--------+-------------+
	//  | Prefix  | id     |height  |  txId       |
	//  |---------+--------+--------+-------------+
	//  | 3 bytes |4 bytes |8 bytes | 32 bytes    |
	//  +---------+--------+--------+-------------+
	//           \|/
	//  +---------+---------------+---------------------+
	//  | shadow  | active height | data n bytes        |
	//  +---------+---------------+---------------------+
	//  | 1 byte  | 8 bytes       | n bytes             |
	//  +---------+---------------+---------------------+
	governKeyLength       = 47
	governSearchKeyLength = 15
)

type governConfig struct {
	id           uint32     // 4 bytes
	blockHeight  uint64     // 8 bytes
	txSha        *wire.Hash // 32 bytes
	shadow       bool       // 1 byte  0 enable | 1 shadow
	activeHeight uint64     // 8 bytes
	data         []byte     // var
	delete       bool       // 1 bytes
}

type governConfigMapKey struct {
	id          uint32     // 4  bytes
	blockHeight uint64     // 8 bytes
	txSha       *wire.Hash // 32 bytes
}

func makeGovernConfigMapKeyToKey(mapKey governConfigMapKey) []byte {
	key := make([]byte, governKeyLength)
	copy(key[0:recordGovernTxLen], recordGovernTx)
	binary.LittleEndian.PutUint32(key[recordGovernTxLen:recordGovernTxLen+4], mapKey.id)
	binary.LittleEndian.PutUint64(key[recordGovernTxLen+4:recordGovernTxLen+12], mapKey.blockHeight)
	copy(key[recordGovernTxLen+12:governKeyLength], mapKey.txSha[:])
	return key
}

func makeGovernConfigSearchKey(id uint32, height uint64) []byte {
	key := make([]byte, governSearchKeyLength)
	copy(key[0:recordGovernTxLen], recordGovernTx)
	binary.LittleEndian.PutUint32(key[recordGovernTxLen:recordGovernTxLen+4], id)
	binary.LittleEndian.PutUint64(key[recordGovernTxLen+4:governSearchKeyLength], height)
	return key
}

func (db *ChainDb) InsertGovernConfig(id uint32, height, activeHeight uint64, shadow bool, txSha *wire.Hash, data []byte) error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()
	return db.insertGovernConfig(id, height, activeHeight, shadow, txSha, data)
}

func (db *ChainDb) fetchGovernConfig(class uint32, height uint64, includeShadow bool) ([]*database.GovernConfig, error) {
	keyPrefix := makeGovernConfigSearchKey(class, height)
	iter := db.localStorage.NewIterator(storage.BytesPrefix(keyPrefix))
	configs := make([]*database.GovernConfig, 0)
	defer iter.Release()
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		if len(value) < 9 {
			continue
		}
		shadow := value[0] != 0x00
		if !includeShadow && shadow {
			continue
		}
		activeHeight := binary.LittleEndian.Uint64(value[1:9])
		data := value[9:]
		blockHeight := binary.LittleEndian.Uint64(key[recordGovernTxLen+4 : recordGovernTxLen+12])
		txSha, err := wire.NewHash(key[recordGovernTxLen+12:])
		if err != nil {
			return nil, err
		}
		configs = append(configs, &database.GovernConfig{
			Id:           class,
			BlockHeight:  blockHeight,
			ActiveHeight: activeHeight,
			Shadow:       shadow,
			TxSha:        txSha,
			Data:         data,
		})
	}
	return configs, nil
}

func (db *ChainDb) insertGovernConfig(id uint32, height, activeHeight uint64, shadow bool, txSha *wire.Hash, data []byte) error {
	key := governConfigMapKey{
		id:          id,
		blockHeight: height,
		txSha:       txSha,
	}
	db.governConfigMap[key] = &governConfig{
		id:           id,
		shadow:       shadow,
		blockHeight:  height,
		activeHeight: activeHeight,
		txSha:        txSha,
		data:         data,
		delete:       false,
	}
	return nil
}

// FetchGovernConfig fetch all config
func (db *ChainDb) FetchGovernConfig(id uint32, height uint64, includeShadow bool) ([]*database.GovernConfig, error) {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()
	return db.fetchGovernConfig(id, height, includeShadow)
}
