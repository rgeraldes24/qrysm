package rpc

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	grpcutil "github.com/theQRL/qrysm/api/grpc"
	"github.com/theQRL/qrysm/validator/client"
	nodeClientFactory "github.com/theQRL/qrysm/validator/client/node-client-factory"
	validatorClientFactory "github.com/theQRL/qrysm/validator/client/validator-client-factory"
	validatorHelpers "github.com/theQRL/qrysm/validator/helpers"
	"google.golang.org/grpc"
)

// registerBeaconClient dials the beacon node and wires the clients used by
// keymanager HTTP/gRPC handlers such as SetVoluntaryExit.
func (s *Server) registerBeaconClient() error {
	dialOpts := client.ConstructDialOptions(
		s.clientMaxCallRecvMsgSize,
		s.clientWithCert,
		s.clientGrpcRetries,
		s.clientGrpcRetryDelay,
	)
	if dialOpts == nil {
		return errors.New("no dial options for beacon chain gRPC client")
	}

	s.ctx = grpcutil.AppendHeaders(s.ctx, s.clientGrpcHeaders)

	grpcConn, err := grpc.DialContext(s.ctx, s.beaconClientEndpoint, dialOpts...)
	if err != nil {
		return errors.Wrapf(err, "could not dial endpoint: %s", s.beaconClientEndpoint)
	}
	if s.clientWithCert != "" {
		log.Info("Established secure gRPC connection")
	}

	conn := validatorHelpers.NewNodeConnection(
		grpcConn,
		s.beaconApiEndpoint,
		s.beaconApiTimeout,
		grpcConn.Close,
	)
	s.beaconConn = conn
	s.beaconNodeClient = nodeClientFactory.NewNodeClient(conn)
	s.beaconNodeValidatorClient = validatorClientFactory.NewValidatorClient(conn)
	log.WithFields(logrus.Fields{
		"beacon-rpc":  s.beaconClientEndpoint,
		"beacon-rest": s.beaconApiEndpoint,
	}).Info("Registered beacon node clients for keymanager API")
	return nil
}
