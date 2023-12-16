package blockchain

// TODO(rgeraldes24) - remove
/*
import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/go-zond/common"
	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/time/slots"
)

// validateMergeBlock validates terminal block hash in the event of manual overrides before checking for total difficulty.
func (s *Service) validateMergeBlock(ctx context.Context, b interfaces.ReadOnlySignedBeaconBlock) error {
	if err := blocks.BeaconBlockIsNil(b); err != nil {
		return err
	}
	payload, err := b.Block().Body().Execution()
	if err != nil {
		return err
	}
	if payload.IsNil() {
		return errors.New("nil execution payload")
	}
	ok, err := canUseValidatedTerminalBlockHash(b.Block().Slot(), payload)
	if err != nil {
		return errors.Wrap(err, "could not validate terminal block hash")
	}
	if ok {
		return nil
	}
	mergeBlockParentHash, mergeBlockTD, err := s.getBlkParentHashAndTD(ctx, payload.ParentHash())
	if err != nil {
		return errors.Wrap(err, "could not get merge block parent hash and total difficulty")
	}
	_, mergeBlockParentTD, err := s.getBlkParentHashAndTD(ctx, mergeBlockParentHash)
	if err != nil {
		return errors.Wrap(err, "could not get merge parent block total difficulty")
	}
	valid, err := validateTerminalBlockDifficulties(mergeBlockTD, mergeBlockParentTD)
	if err != nil {
		return err
	}
	if !valid {
		err := fmt.Errorf("invalid TTD, configTTD: %s, currentTTD: %s, parentTTD: %s",
			params.BeaconConfig().TerminalTotalDifficulty, mergeBlockTD, mergeBlockParentTD)
		return invalidBlock{error: err}
	}

	log.WithFields(logrus.Fields{
		"slot":                            b.Block().Slot(),
		"mergeBlockHash":                  common.BytesToHash(payload.ParentHash()).String(),
		"mergeBlockParentHash":            common.BytesToHash(mergeBlockParentHash).String(),
		"terminalTotalDifficulty":         params.BeaconConfig().TerminalTotalDifficulty,
		"mergeBlockTotalDifficulty":       mergeBlockTD,
		"mergeBlockParentTotalDifficulty": mergeBlockParentTD,
	}).Info("Validated terminal block")

	log.Info(mergeAsciiArt)

	return nil
}

// getBlkParentHashAndTD retrieves the parent hash and total difficulty of the given block.
func (s *Service) getBlkParentHashAndTD(ctx context.Context, blkHash []byte) ([]byte, *uint256.Int, error) {
	blk, err := s.cfg.ExecutionEngineCaller.ExecutionBlockByHash(ctx, common.BytesToHash(blkHash), false //no txs )
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get pow block")
	}
	if blk == nil {
		return nil, nil, errors.New("pow block is nil")
	}
	// TODO(rgeraldes24)
	//blk.Version = version.Bellatrix
	blkTDBig, err := hexutil.DecodeBig(blk.TotalDifficulty)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not decode merge block total difficulty")
	}
	blkTDUint256, overflows := uint256.FromBig(blkTDBig)
	if overflows {
		return nil, nil, errors.New("total difficulty overflows")
	}
	return blk.ParentHash[:], blkTDUint256, nil
}

// canUseValidatedTerminalBlockHash validates if the merge block is a valid terminal PoW block.
func canUseValidatedTerminalBlockHash(blkSlot primitives.Slot, payload interfaces.ExecutionData) (bool, error) {
	if bytesutil.ToBytes32(params.BeaconConfig().TerminalBlockHash.Bytes()) == [32]byte{} {
		return false, nil
	}
	if params.BeaconConfig().TerminalBlockHashActivationEpoch > slots.ToEpoch(blkSlot) {
		return false, errors.New("terminal block hash activation epoch not reached")
	}
	if !bytes.Equal(payload.ParentHash(), params.BeaconConfig().TerminalBlockHash.Bytes()) {
		return false, errors.New("parent hash does not match terminal block hash")
	}
	return true, nil
}

// validateTerminalBlockDifficulties validates terminal pow block by comparing own total difficulty with parent's total difficulty.
func validateTerminalBlockDifficulties(currentDifficulty *uint256.Int, parentDifficulty *uint256.Int) (bool, error) {
	b, ok := new(big.Int).SetString(params.BeaconConfig().TerminalTotalDifficulty, 10)
	if !ok {
		return false, errors.New("failed to parse terminal total difficulty")
	}
	ttd, of := uint256.FromBig(b)
	if of {
		return false, errors.New("overflow terminal total difficulty")
	}
	totalDifficultyReached := currentDifficulty.Cmp(ttd) >= 0
	parentTotalDifficultyValid := ttd.Cmp(parentDifficulty) > 0
	return totalDifficultyReached && parentTotalDifficultyValid, nil
}
*/
