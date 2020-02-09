package tss

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/binance-chain/tss-lib/ecdsa/keygen"

	"gitlab.com/thorchain/tss/go-tss/common"
)

// saveLocalStateToFile is going to persistent the local state into a file
func saveLocalStateToFile(filePathName string, state common.KeygenLocalStateItem) error {
	buf, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("fail to marshal KeygenLocalState to json: %w", err)
	}
	return ioutil.WriteFile(filePathName, buf, 0655)
}

func saveKeygenResult(filePath string, data keygen.LocalPartySaveData, keyGenLocalStateItem common.KeygenLocalStateItem) error {
	pubKey, err := getThorPubKey(data.ECDSAPub)
	if err != nil {
		return fmt.Errorf("fail to get thorchain pubkey: %w", err)
	}
	keyGenLocalStateItem.PubKey = pubKey
	keyGenLocalStateItem.LocalData = data
	localFileName := fmt.Sprintf("localstate-%s.json", pubKey)
	if len(filePath) > 0 {
		localFileName = filepath.Join(filePath, localFileName)
	}
	if path.Dir(filePath) == "." {
		return fmt.Errorf("error path(%s) not exist", filePath)
	}
	return saveLocalStateToFile(localFileName, keyGenLocalStateItem)
}
