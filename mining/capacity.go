package mining

import (
	"reflect"

	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/poc/engine"
	_ "github.com/Sukhavati-Labs/go-miner/poc/engine/pocminer/miner"
	"github.com/Sukhavati-Labs/go-miner/poc/engine/spacekeeper"
	"github.com/Sukhavati-Labs/go-miner/poc/engine/spacekeeper/capacity"
)

type SpaceKeeper interface {
	spacekeeper.SpaceKeeper
	Configured() bool
	ConfigureByBitLength(BlCount map[int]int, execPlot, execMine bool, cointype uint32) ([]engine.WorkSpaceInfo, error)
	ConfigureBySize(targetSize uint64, execPlot, execMine bool, cointype uint32) ([]engine.WorkSpaceInfo, error)
	ConfigureByPath(paths []string, sizes []uint64, execPlot, execMine, autoCreate bool, cointype uint32) ([]engine.WorkSpaceInfo, error)
	AvailableDiskSize() (uint64, error)
	IsCapacityAvailable(path string, capacity uint64) error
	WorkSpaceInfosByDirs() (dirs []string, results [][]engine.WorkSpaceInfo, err error)
}

type ConfigurableSpaceKeeper struct {
	spacekeeper.SpaceKeeper
}

func NewConfigurableSpaceKeeper(sk spacekeeper.SpaceKeeper) *ConfigurableSpaceKeeper {
	return &ConfigurableSpaceKeeper{sk}
}

func (csk *ConfigurableSpaceKeeper) Configured() bool {
	sk, err := getInstance(csk.SpaceKeeper)
	if err != nil {
		return true
	}
	return sk.Configured()
}

func (csk *ConfigurableSpaceKeeper) ConfigureByBitLength(BlCount map[int]int, execPlot, execMine bool, cointype uint32) ([]engine.WorkSpaceInfo, error) {
	sk, err := getInstance(csk.SpaceKeeper)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail to assert SpaceKeeper type", logging.LogFormat{"actual": reflect.TypeOf(sk)})
		return nil, err
	}
	return sk.ConfigureByBitLength(BlCount, execPlot, execMine, cointype)
}

func (csk *ConfigurableSpaceKeeper) ConfigureBySize(targetSize uint64, execPlot, execMine bool, cointype uint32) ([]engine.WorkSpaceInfo, error) {
	sk, err := getInstance(csk.SpaceKeeper)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail to assert SpaceKeeper type", logging.LogFormat{"actual": reflect.TypeOf(sk)})
		return nil, err
	}
	return sk.ConfigureBySize(targetSize, execPlot, execMine, cointype)
}

func (csk *ConfigurableSpaceKeeper) ConfigureByPath(paths []string, sizes []uint64, execPlot, execMine, autoCreate bool, cointype uint32) ([]engine.WorkSpaceInfo, error) {
	sk, err := getInstance(csk.SpaceKeeper)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail to assert SpaceKeeper type", logging.LogFormat{"actual": reflect.TypeOf(sk)})
		return nil, err
	}
	sizesInt := make([]int, len(sizes))
	for i := range sizes {
		sizesInt[i] = int(sizes[i])
	}
	return sk.ConfigureByPath(paths, sizesInt, execPlot, execMine, autoCreate, cointype)
}

func (csk *ConfigurableSpaceKeeper) AvailableDiskSize() (uint64, error) {
	sk, err := getInstance(csk.SpaceKeeper)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail to assert SpaceKeeper type", logging.LogFormat{"actual": reflect.TypeOf(sk)})
		return 0, err
	}
	return sk.AvailableDiskSize(), nil
}

func (csk *ConfigurableSpaceKeeper) IsCapacityAvailable(path string, capacity uint64) error {
	sk, err := getInstance(csk.SpaceKeeper)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail to assert SpaceKeeper type", logging.LogFormat{"actual": reflect.TypeOf(sk)})
		return err
	}
	return sk.IsCapacityAvailable(path, capacity)
}

func (csk *ConfigurableSpaceKeeper) WorkSpaceInfosByDirs() (dirs []string, results [][]engine.WorkSpaceInfo, err error) {
	sk, err := getInstance(csk.SpaceKeeper)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail to assert SpaceKeeper type", logging.LogFormat{"actual": reflect.TypeOf(sk)})
		return nil, nil, err
	}
	return sk.WorkSpaceInfosByDirs()
}

func getInstance(sk spacekeeper.SpaceKeeper) (*capacity.SpaceKeeper, error) {
	ins, ok := sk.(*capacity.SpaceKeeper)
	if !ok {
		return nil, spacekeeper.ErrUnimplemented
	}
	return ins, nil
}
