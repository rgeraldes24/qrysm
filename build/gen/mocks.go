package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// mockgenPkg is the mockgen the checked-in mocks come from, built at the
// version go.mod selects.
const mockgenPkg = "github.com/golang/mock/mockgen"

// reflectMock is a mock generated in reflect mode from a package's interfaces.
type reflectMock struct {
	dest, pkg, importPath, interfaces string
}

// sourceMock is a mock generated in source mode from a Go file.
type sourceMock struct {
	dest, pkg, source string
}

// mockGroup is a set of mocks generated together, then run through goimports
// and gofmt -s when format is set.
type mockGroup struct {
	reflect []reflectMock
	source  []sourceMock
	format  []string
}

// The mocks hack/update-mockgen.sh generated. The other mocks checked in under
// testing/mock (beacon_altair_validator_{client,server}_mock.go) and
// cmd/mock are maintained by hand.
var mockGroups = []mockGroup{
	{
		reflect: []reflectMock{
			{"testing/mock/beacon_service_mock.go", "mock", "github.com/theQRL/qrysm/proto/qrysm/v1alpha1", "BeaconChainClient"},
			{"testing/mock/beacon_validator_server_mock.go", "mock", "github.com/theQRL/qrysm/proto/qrysm/v1alpha1", "BeaconNodeValidatorServer,BeaconNodeValidator_WaitForActivationServer,BeaconNodeValidator_WaitForChainStartServer"},
			{"testing/mock/beacon_validator_client_mock.go", "mock", "github.com/theQRL/qrysm/proto/qrysm/v1alpha1", "BeaconNodeValidatorClient,BeaconNodeValidator_WaitForChainStartClient,BeaconNodeValidator_WaitForActivationClient"},
			{"testing/mock/node_service_mock.go", "mock", "github.com/theQRL/qrysm/proto/qrysm/v1alpha1", "NodeClient"},
			{"testing/mock/event_service_mock.go", "mock", "github.com/theQRL/qrysm/proto/qrl/service", "EventsClient,Events_StreamEventsClient,Events_StreamEventsServer"},
			{"testing/mock/keymanager_mock.go", "mock", "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client", "RemoteSignerClient"},
			{"testing/validator-mock/beacon_chain_client_mock.go", "validator_mock", "github.com/theQRL/qrysm/validator/client/iface", "BeaconChainClient"},
			{"testing/validator-mock/node_client_mock.go", "validator_mock", "github.com/theQRL/qrysm/validator/client/iface", "NodeClient"},
			{"testing/validator-mock/validator_client_mock.go", "validator_mock", "github.com/theQRL/qrysm/validator/client/iface", "ValidatorClient"},
		},
		format: []string{"testing/mock"},
	},
	{
		source: []sourceMock{
			{"validator/client/beacon-api/mock/genesis_mock.go", "mock", "validator/client/beacon-api/genesis.go"},
			{"validator/client/beacon-api/mock/duties_mock.go", "mock", "validator/client/beacon-api/duties.go"},
			{"validator/client/beacon-api/mock/json_rest_handler_mock.go", "mock", "validator/client/beacon-api/json_rest_handler.go"},
			{"validator/client/beacon-api/mock/state_validators_mock.go", "mock", "validator/client/beacon-api/state_validators.go"},
			{"validator/client/beacon-api/mock/beacon_block_converter_mock.go", "mock", "validator/client/beacon-api/beacon_block_converter.go"},
		},
		format: []string{"validator/client/beacon-api/mock"},
	},
	{
		source: []sourceMock{
			{"crypto/ml_dsa_87/common/mock/interface_mock.go", "mock", "crypto/ml_dsa_87/common/interface.go"},
		},
	},
}

func genMocks() error {
	tmpRoot, err := os.MkdirTemp("", "gen-mocks-")
	if err != nil {
		return fmt.Errorf("mkdirTemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	mockgen := filepath.Join(tmpRoot, "mockgen")
	if err := sh("go", "build", "-o", mockgen, mockgenPkg); err != nil {
		return fmt.Errorf("build mockgen: %w", err)
	}

	for _, g := range mockGroups {
		for _, m := range g.reflect {
			fmt.Printf("generating %s for interfaces: %s\n", m.dest, m.interfaces)
			if err := sh(mockgen, "-package="+m.pkg, "-destination="+m.dest, m.importPath, m.interfaces); err != nil {
				return fmt.Errorf("mockgen: %w", err)
			}
		}

		for _, m := range g.source {
			fmt.Printf("generating %s for file: %s\n", m.dest, m.source)
			if err := sh(mockgen, "-package="+m.pkg, "-source="+m.source, "-destination="+m.dest); err != nil {
				return fmt.Errorf("mockgen: %w", err)
			}
		}

		if len(g.format) == 0 {
			continue
		}

		if err := goimports(g.format...); err != nil {
			return fmt.Errorf("goimports: %w", err)
		}

		if err := gofmtSimplify(g.format...); err != nil {
			return fmt.Errorf("gofmtSimplify: %w", err)
		}
	}

	return nil
}
