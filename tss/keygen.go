package tss

import (
	"sync/atomic"

	"gitlab.com/thorchain/tss/go-tss/common"
	"gitlab.com/thorchain/tss/go-tss/keygen"
)

func (t *TssServer) Keygen(req keygen.Request) (keygen.Response, error) {
	t.tssKeyGenLocker.Lock()
	defer t.tssKeyGenLocker.Unlock()

	status := common.Success

	msgID, err := t.requestToMsgId(req)
	if err != nil {
		return keygen.Response{}, err
	}

	// the statistic of keygen only care about Tss it self, even if the
	// following http response aborts, it still counted as a successful keygen
	// as the Tss model runs successfully.
	k, err := t.keygenInstance.GenerateNewKey(req, msgID)
	if err != nil {
		t.logger.Error().Err(err).Msg("err in keygen")
		atomic.AddUint64(&t.Status.FailedKeyGen, 1)
	} else {
		atomic.AddUint64(&t.Status.SucKeyGen, 1)
	}

	newPubKey, addr, err := common.GetTssPubKey(k)
	if err != nil {
		t.logger.Error().Err(err).Msg("fail to generate the new Tss key")
		status = common.Fail
	}

	return keygen.NewResponse(
		newPubKey,
		addr.String(),
		status,
		common.NoBlame,
	), nil
}
