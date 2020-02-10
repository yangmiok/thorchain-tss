package tss

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"gitlab.com/thorchain/tss/go-tss/common"
)

// LocalStateManager provide necessary methods to manage the local state, save the keygen result to file , load it from file etc
type LocalStateManager interface {
}

func SaveLocalStateToFile(filePathName string, state common.KeygenLocalStateItem) error {
	buf, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("fail to marshal KeygenLocalState to json: %w", err)
	}
	return ioutil.WriteFile(filePathName, buf, 0655)
}
