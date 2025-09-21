package sync

import (
	"context"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/core/feed"
	opfeed "github.com/theQRL/qrysm/beacon-chain/core/feed/operation"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"google.golang.org/protobuf/proto"
)

func (s *Service) mlDSA87ToExecutionChangeSubscriber(_ context.Context, msg proto.Message) error {
	mlDSA87Msg, ok := msg.(*qrysmpb.SignedMLDSA87ToExecutionChange)
	if !ok {
		return errors.Errorf("incorrect type of message received, wanted %T but got %T", &qrysmpb.SignedMLDSA87ToExecutionChange{}, msg)
	}
	s.cfg.operationNotifier.OperationFeed().Send(&feed.Event{
		Type: opfeed.MLDSA87ToExecutionChangeReceived,
		Data: &opfeed.MLDSA87ToExecutionChangeReceivedData{
			Change: mlDSA87Msg,
		},
	})
	s.cfg.mlDSA87ToExecPool.InsertMLDSA87ToExecChange(mlDSA87Msg)
	return nil
}
