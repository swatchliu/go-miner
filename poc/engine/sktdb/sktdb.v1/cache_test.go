package sktdb_v1_test

import (
	"encoding/json"
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/poc"
	"github.com/Sukhavati-Labs/go-miner/poc/engine/sktdb/sktdb.v1"
	"github.com/Sukhavati-Labs/go-miner/testutil"
	"github.com/shirou/gopsutil/mem"
	"math/rand"
	"runtime"
	"testing"
)

func TestFreeMemory(t *testing.T) {
	testutil.SkipCI(t)

	var requiredMem = 4 * 1024 * poc.MiB
	var round = 10

	for i := 0; i < round; i++ {
		mockCache(requiredMem, t)
	}
}

func mockCache(requiredMem int, t *testing.T) {
	if m, err := mem.VirtualMemory(); err != nil {
		t.Fatal(err)
	} else if uint64(requiredMem) > m.Available {
		t.Skip("memory is not enough for test", requiredMem, m.Available)
	}

	printMemory(t)

	cache := sktdb_v1.NewMemCache(requiredMem)
	if err := fillCache(cache); err != nil {
		t.Fatal(err)
	}

	printMemory(t)

	cache.Release()

	printMemory(t)
}

func fillCache(cache *sktdb_v1.MemCache) error {
	var buf [poc.MiB]byte

	for i := 0; i < cache.Len(); i += len(buf) {
		if _, err := rand.Read(buf[:]); err != nil {
			return err
		}
		cache.WriteAt(buf[:], int64(i))
	}
	return nil
}

func printMemory(t *testing.T) {
	stat := &runtime.MemStats{}
	runtime.ReadMemStats(stat)
	if str, err := json.Marshal(stat); err != nil {
		t.Fatal(err)
	} else {
		fmt.Println(string(str))
	}

	stat2, err := mem.VirtualMemory()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("\n", stat2.String())
}
