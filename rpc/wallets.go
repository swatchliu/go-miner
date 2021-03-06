package rpc

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/poc/wallet"
	"github.com/Sukhavati-Labs/go-miner/poc/wallet/keystore"
	"io"
	"os"

	"github.com/Sukhavati-Labs/go-miner/logging"
	pb "github.com/Sukhavati-Labs/go-miner/rpc/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/status"
)

var (
	keystoreFileNamePrefix = "keystore"
)

func (s *Server) GetKeystore(ctx context.Context, msg *empty.Empty) (*pb.GetKeystoreResponse, error) {
	keystores := make([]*pb.WalletSummary, 0)
	for _, summary := range s.pocWallet.GetManagedAddrManager() {
		keystores = append(keystores, &pb.WalletSummary{
			WalletId: summary.Name(),
			Remark:   summary.Remarks(),
		})
	}
	return &pb.GetKeystoreResponse{
		Wallets: keystores,
	}, nil
}

func getKeystoreDetail(keystore *keystore.Keystore) *pb.PocWallet {

	walletCrypto := &pb.WalletCrypto{
		Cipher:             keystore.Crypto.Cipher,
		MasterHDPrivKeyEnc: keystore.Crypto.MasterHDPrivKeyEnc,
		KDF:                keystore.Crypto.KDF,
		PubParams:          keystore.Crypto.PubParams,
		PrivParams:         keystore.Crypto.PrivParams,
		CryptoKeyPubEnc:    keystore.Crypto.CryptoKeyPubEnc,
		CryptoKeyPrivEnc:   keystore.Crypto.CryptoKeyPrivEnc,
	}
	HdWalletPath := &pb.HDWalletPath{
		Purpose:          keystore.HDpath.Purpose,
		Cointype:         keystore.HDpath.Coin,
		Account:          keystore.HDpath.Account,
		ExternalChildNum: keystore.HDpath.ExternalChildNum,
		InternalChildNum: keystore.HDpath.InternalChildNum,
	}

	return &pb.PocWallet{
		Remark: keystore.Remark,
		Crypto: walletCrypto,
		HDPath: HdWalletPath,
	}
}
func getAddrManagerDetail(addrManager *keystore.AddrManager) *pb.AddrManager {
	addresses := make([]*pb.PocAddress, 0)
	for _, addr := range addrManager.ManagedAddresses() {
		HdPath := addr.DerivationPath()
		address := &pb.PocAddress{
			PubKey:     hex.EncodeToString(addr.PubKey().SerializeCompressed()),
			ScriptHash: hex.EncodeToString(addr.ScriptAddress()),
			Address:    addr.String(),
			DerivationPath: &pb.DerivationPath{
				Account: HdPath.Account,
				Branch:  HdPath.Branch,
				Index:   HdPath.Index,
			},
		}
		// The private key is not visible
		//if addr.PrivKey() != nil {
		//	address.PrivKey = hex.EncodeToString(addr.PrivKey().Serialize())
		//}
		addresses = append(addresses, address)
	}
	return &pb.AddrManager{
		KeystoreName: addrManager.Name(),
		Remark:       addrManager.Remarks(),
		Expires:      0,
		Addresses:    addresses,
		Use:          uint32(addrManager.AddrUse()),
	}
}

func (s *Server) GetKeystoreDetail(ctx context.Context, in *pb.GetKeystoreDetailRequest) (*pb.GetKeystoreDetailResponse, error) {
	keystore, addrManager, err := s.pocWallet.ExportKeystore(in.WalletId, []byte(in.Passphrase))
	if err != nil {
		logging.CPrint(logging.ERROR, "GetKeystoreDetail ExportKeystore error", logging.LogFormat{"error": err})
		return nil, err
	}
	detail := getKeystoreDetail(keystore)
	detail.WalletId = in.WalletId
	detail.AddrManager = getAddrManagerDetail(addrManager)
	return &pb.GetKeystoreDetailResponse{
		WalletId: in.WalletId,
		Wallet:   detail,
	}, nil
}

func (s *Server) ExportKeystore(ctx context.Context, in *pb.ExportKeystoreRequest) (*pb.ExportKeystoreResponse, error) {
	logging.CPrint(logging.INFO, "a request is received to export keystore", logging.LogFormat{"export keystore id": in.WalletId})
	err := checkWalletIdLen(in.WalletId)
	if err != nil {
		logging.CPrint(logging.ERROR, "ExportKeystore  checkWalletIdLen error", logging.LogFormat{"error": err})
		return nil, err
	}
	err = checkPassLen(in.Passphrase)
	if err != nil {
		logging.CPrint(logging.ERROR, "ExportKeystore  checkPassLen error", logging.LogFormat{"error": err})
		return nil, err
	}
	// get keystore json from wallet
	keystore, addrManager, err := s.pocWallet.ExportKeystore(in.WalletId, []byte(in.Passphrase))
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIExportWallet], logging.LogFormat{
			"err": err,
		})
		return nil, status.New(ErrAPIExportWallet, ErrCode[ErrAPIExportWallet]).Err()
	}
	keystoreJSON := keystore.Bytes()
	// write keystore json file to disk
	exportFileName := fmt.Sprintf("%s/%s-%s.json", in.ExportPath, keystoreFileNamePrefix, in.WalletId)
	file, err := os.OpenFile(exportFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIOpenFile], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIOpenFile, ErrCode[ErrAPIOpenFile]).Err()
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	_, err = writer.WriteString(string(keystoreJSON))
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIWriteFile], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIWriteFile, ErrCode[ErrAPIWriteFile]).Err()
	}
	err = writer.Flush()
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIFlush], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIFlush, ErrCode[ErrAPIFlush]).Err()
	}

	logging.CPrint(logging.INFO, "the request to export keystore was successfully answered", logging.LogFormat{"export keystore id": in.WalletId})
	return &pb.ExportKeystoreResponse{
		Keystore:    string(keystoreJSON),
		AddrManager: getAddrManagerDetail(addrManager),
	}, nil
}

func pushJsonFile(exportFileName string, data []byte) error {
	file, err := os.OpenFile(exportFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIOpenFile], logging.LogFormat{
			"error": err,
		})
		return status.New(ErrAPIOpenFile, ErrCode[ErrAPIOpenFile]).Err()
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	_, err = writer.WriteString(string(data))
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIWriteFile], logging.LogFormat{
			"error": err,
		})
		return status.New(ErrAPIWriteFile, ErrCode[ErrAPIWriteFile]).Err()
	}
	err = writer.Flush()
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIFlush], logging.LogFormat{
			"error": err,
		})
		return status.New(ErrAPIFlush, ErrCode[ErrAPIFlush]).Err()
	}
	return nil
}

func (s *Server) ExportKeystoreByDir(ctx context.Context, in *pb.ExportKeystoreByDirRequest) (*pb.ExportKeystoreByDirResponse, error) {
	logging.CPrint(logging.INFO, "rpc ExportKeystoreByDirs called")
	pocWalletConfig := wallet.NewPocWalletConfig(in.WalletDir, "leveldb")
	exportWallet, err := wallet.OpenPocWallet(pocWalletConfig, []byte(in.Passphrase))
	if err != nil {
		logging.CPrint(logging.ERROR, "ExportKeystoreByDir  OpenPocWallet error", logging.LogFormat{"error": err})
		return nil, err
	}
	defer exportWallet.Close()
	keystores, managers, err := exportWallet.ExportKeystores([]byte(in.WalletPassphrase))
	if err != nil {
		logging.CPrint(logging.ERROR, "ExportKeystoreByDir  ExportKeystores error", logging.LogFormat{"error": err})
		return nil, err
	}
	keystoreJSON, err := json.Marshal(keystores)
	if err != nil {
		logging.CPrint(logging.ERROR, "ExportKeystoreByDir  json Marshal keystores error", logging.LogFormat{"error": err})
		return nil, err
	}
	//addrManagersJson, err := json.Marshal(addrManagers)
	addrManagers := make([]*pb.AddrManager, 0)
	for _, a := range managers {
		addrManagers = append(addrManagers, getAddrManagerDetail(a))
	}
	// write keystore json file to disk
	exportFileName := fmt.Sprintf("%s/%s-all.json", in.ExportPath, keystoreFileNamePrefix)
	//exportAddrsFileName := fmt.Sprintf("%s/%s-addrs.json", in.ExportPath, keystoreFileNamePrefix)
	err = pushJsonFile(exportFileName, keystoreJSON)
	if err != nil {
		logging.CPrint(logging.ERROR, "ExportKeystoreByDir pushJsonFile keystores error",
			logging.LogFormat{"error": err, "file name": exportFileName})
		return nil, err
	}
	//err = pushJsonFile(exportAddrsFileName, addrManagersJson)
	//if err != nil {
	//	return nil, err
	//}
	logging.CPrint(logging.INFO, "the request to export keystore was successfully answered")
	return &pb.ExportKeystoreByDirResponse{
		Keystores:    string(keystoreJSON),
		AddrManagers: addrManagers,
	}, nil
}

func (s *Server) ImportKeystore(ctx context.Context, in *pb.ImportKeystoreRequest) (*pb.ImportKeystoreResponse, error) {
	err := checkPassLen(in.OldPassphrase)
	if err != nil {
		logging.CPrint(logging.ERROR, "ImportKeystore checkPassLen OldPassphrase",
			logging.LogFormat{"error": err})
		return nil, err
	}
	if len(in.NewPassphrase) != 0 {
		err = checkPassLen(in.NewPassphrase)
		if err != nil {
			logging.CPrint(logging.ERROR, "ImportKeystore checkPassLen NewPassphrase",
				logging.LogFormat{"error": err})
			return nil, err
		}
	}

	file, err := os.Open(in.ImportPath)
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to open file", logging.LogFormat{"file": in.ImportPath, "error": err})
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	fileBytes, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		logging.CPrint(logging.ERROR, "failed to read file", logging.LogFormat{"error": err})
		return nil, err
	}

	accountID, remark, err := s.pocWallet.ImportKeystore(fileBytes, []byte(in.OldPassphrase), []byte(in.NewPassphrase))
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to import keystore", logging.LogFormat{"error": err})
		return nil, err
	}
	return &pb.ImportKeystoreResponse{
		Status:   true,
		WalletId: accountID,
		Remark:   remark,
	}, nil
}

func (s *Server) ImportKeystoreByDir(ctx context.Context, in *pb.ImportKeystoreByDirRequest) (*pb.ImportKeystoreByDirResponse, error) {
	logging.CPrint(logging.INFO, "rpc ImportKeystoreByDirs called")
	if s.pocWallet.IsLocked() {
		err := s.pocWallet.Unlock([]byte(in.CurrentPrivpass))
		if err != nil {
			return nil, err
		}
	}
	pocWalletConfig := wallet.NewPocWalletConfig(in.ImportKeystoreDir, "leveldb")
	exportWallet, err := wallet.OpenPocWallet(pocWalletConfig, []byte(in.ImportPubpass))
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to ImportKeystoreByDir OpenPocWallet", logging.LogFormat{"error": err})
		return nil, err
	}
	defer exportWallet.Close()
	keystores, oldManagers, err := exportWallet.ExportKeystores([]byte(in.ImportPrivpass))
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to ImportKeystoreByDir ExportKeystores", logging.LogFormat{"error": err})
		return nil, err
	}
	oldAddressManager := make([]*pb.AddrManager, 0)
	for _, a := range oldManagers {
		oldAddressManager = append(oldAddressManager, getAddrManagerDetail(a))
	}
	newAddressManager := make([]*pb.AddrManager, 0)
	resp := &pb.ImportKeystoreByDirResponse{}
	resp.Keystores = make(map[string]*pb.PocWallet)
	for _, ks := range keystores {
		walletId, _, err := s.pocWallet.ImportKeystore(ks.Bytes(), []byte(in.ImportPrivpass), []byte(in.CurrentPrivpass))
		if err != nil {
			logging.CPrint(logging.ERROR, "failed to ImportKeystoreByDir ImportKeystore", logging.LogFormat{"error": err})
			return nil, err
		}
		manager, b := s.pocWallet.GetAddrManager(walletId)
		if !b {
			logging.CPrint(logging.ERROR, "failed to ImportKeystoreByDir ImportKeystore GetAddrManager", logging.LogFormat{"error": err})
			return nil, fmt.Errorf("Import With Error ")
		}

		newAddressManager = append(newAddressManager, getAddrManagerDetail(manager))
	}
	keystores, _, err = s.pocWallet.ExportKeystores([]byte(in.CurrentPrivpass))
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to ImportKeystoreByDir ExportKeystores", logging.LogFormat{"error": err})
		return nil, err
	}
	for walletId, ks := range keystores {
		resp.Keystores[walletId] = getKeystoreDetail(ks)
		manager, b := s.pocWallet.GetAddrManager(walletId)
		if b {
			resp.Keystores[walletId].AddrManager = getAddrManagerDetail(manager)
			resp.Keystores[walletId].WalletId = walletId
		}
	}
	resp.NewAddrManagers = newAddressManager
	resp.OldAddrManagers = oldAddressManager
	return resp, nil
}

func (s *Server) UnlockWallet(ctx context.Context, in *pb.UnlockWalletRequest) (*pb.UnlockWalletResponse, error) {
	logging.CPrint(logging.INFO, "rpc unlock wallet called")
	err := checkPassLen(in.Passphrase)
	if err != nil {
		return nil, err
	}
	resp := &pb.UnlockWalletResponse{}

	if !s.pocWallet.IsLocked() {
		logging.CPrint(logging.INFO, "rpc unlock wallet succeed yet")
		resp.Success, resp.Error = true, ""
		return resp, nil
	}

	if err := s.pocWallet.Unlock([]byte(in.Passphrase)); err != nil {
		logging.CPrint(logging.ERROR, "rpc unlock wallet failed", logging.LogFormat{"err": err})
		return nil, status.New(ErrAPIWalletInternal, err.Error()).Err()
	}

	logging.CPrint(logging.INFO, "rpc unlock wallet succeed")
	resp.Success, resp.Error = true, ""
	return resp, nil
}

func (s *Server) LockWallet(ctx context.Context, msg *empty.Empty) (*pb.LockWalletResponse, error) {
	logging.CPrint(logging.INFO, "rpc lock wallet called")
	resp := &pb.LockWalletResponse{}

	if s.pocWallet.IsLocked() {
		logging.CPrint(logging.INFO, "rpc lock wallet succeed")
		resp.Success, resp.Error = true, ""
		return resp, nil
	}

	if s.pocMiner.Started() {
		logging.CPrint(logging.WARN, "rpc lock wallet failed", logging.LogFormat{"err": ErrCode[ErrAPIWalletIsMining]})
		return nil, status.New(ErrAPIWalletIsMining, ErrCode[ErrAPIWalletIsMining]).Err()
	}

	s.pocWallet.Lock()
	logging.CPrint(logging.INFO, "rpc lock wallet succeed")
	resp.Success, resp.Error = true, ""
	return resp, nil
}

func (s *Server) ChangePrivatePass(ctx context.Context, in *pb.ChangePrivatePassRequest) (*pb.ChangePrivatePassResponse, error) {
	err := checkPassLen(in.OldPrivpass)
	if err != nil {
		logging.CPrint(logging.WARN, "failed to ChangePrivatePass checkPassLen OldPrivpass", logging.LogFormat{"error": err})
		return nil, err
	}
	err = checkPassLen(in.NewPrivpass)
	if err != nil {
		logging.CPrint(logging.WARN, "failed to ChangePrivatePass checkPassLen NewPrivpass", logging.LogFormat{"error": err})
		return nil, err
	}
	err = s.pocWallet.ChangePrivPassphrase([]byte(in.OldPrivpass), []byte(in.NewPrivpass), nil)
	if err != nil {
		logging.CPrint(logging.WARN, "failed to ChangePrivatePass ChangePrivPassphrase", logging.LogFormat{"error": err})
		return nil, err
	}
	return &pb.ChangePrivatePassResponse{
		Success: true,
	}, nil
}

func (s *Server) ChangePublicPass(ctx context.Context, in *pb.ChangePublicPassRequest) (*pb.ChangePublicPassResponse, error) {
	err := checkPassLen(in.OldPubpass)
	if err != nil {
		logging.CPrint(logging.WARN, "failed to ChangePublicPass checkPassLen OldPubpass", logging.LogFormat{"error": err})
		return nil, err
	}
	err = checkPassLen(in.NewPubpass)
	if err != nil {
		logging.CPrint(logging.WARN, "failed to ChangePublicPass checkPassLen NewPubpass", logging.LogFormat{"error": err})
		return nil, err
	}
	err = s.pocWallet.ChangePubPassphrase([]byte(in.OldPubpass), []byte(in.NewPubpass), nil)
	if err != nil {
		logging.CPrint(logging.WARN, "failed to ChangePublicPass ChangePubPassphrase", logging.LogFormat{"error": err})
		return nil, err
	}
	return &pb.ChangePublicPassResponse{
		Success: true,
	}, nil
}
