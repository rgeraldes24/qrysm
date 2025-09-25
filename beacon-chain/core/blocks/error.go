package blocks

import "github.com/pkg/errors"

var errNilSignedWithdrawalMessage = errors.New("nil SignedMLDSA87ToExecutionChange message")
var errNilWithdrawalMessage = errors.New("nil MLDSA87ToExecutionChange message")
var errInvalidMLDSA87Prefix = errors.New("withdrawal credential prefix is not a ml-dsa-87 prefix")
var errInvalidWithdrawalCredentials = errors.New("withdrawal credentials do not match")
