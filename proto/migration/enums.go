package migration

import (
	"github.com/pkg/errors"
	qrysmpb "github.com/theQRL/qrysm/proto/qrl/v1"
	zond "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

func V1Alpha1ConnectionStateToV1(connState zond.ConnectionState) qrysmpb.ConnectionState {
	alphaString := connState.String()
	v1Value := qrysmpb.ConnectionState_value[alphaString]
	return qrysmpb.ConnectionState(v1Value)
}

func V1Alpha1PeerDirectionToV1(peerDirection zond.PeerDirection) (qrysmpb.PeerDirection, error) {
	alphaString := peerDirection.String()
	if alphaString == zond.PeerDirection_UNKNOWN.String() {
		return 0, errors.New("peer direction unknown")
	}
	v1Value := qrysmpb.PeerDirection_value[alphaString]
	return qrysmpb.PeerDirection(v1Value), nil
}
