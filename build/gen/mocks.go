package main

import (
	"fmt"
)

// mockgen runs the go.mod pinned github.com/golang/mock mockgen.
func mockgen(args ...string) error {
	return sh("go", append([]string{"run", "github.com/golang/mock/mockgen"}, args...)...)
}

// reflectMock is a reflection-based mock of interfaces of a package.
type reflectMock struct {
	dest, pkg, importPath, interfaces string
}

// sourceMock is a mock of the interfaces declared in a source file.
type sourceMock struct {
	dest, pkg, source string
}

// mockGroup is a set of mocks written to the same directory.
type mockGroup struct {
	dir     string
	reflect []reflectMock
	source  []sourceMock
	// format runs goimports and gofmt -s over dir once generated.
	format bool
}

func (g mockGroup) dests() []string {
	dests := make([]string, 0, len(g.reflect)+len(g.source))
	for _, m := range g.reflect {
		dests = append(dests, m.dest)
	}

	for _, m := range g.source {
		dests = append(dests, m.dest)
	}

	return dests
}

func mockGroups() []mockGroup {
	const (
		mockPath          = "testing/mock"
		ifaceMockPath     = "testing/validator-mock"
		beaconAPIMockPath = "validator/client/beacon-api/mock"
		mlDSA87MockPath   = "crypto/ml_dsa_87/common/mock"

		v1alpha1Pkg        = modulePath + "/proto/qrysm/v1alpha1"
		servicePkg         = modulePath + "/proto/qrl/service"
		validatorClientPkg = modulePath + "/proto/qrysm/v1alpha1/validator-client"
		ifacePkg           = modulePath + "/validator/client/iface"
	)

	return []mockGroup{
		{
			dir: mockPath,
			reflect: []reflectMock{
				{mockPath + "/beacon_service_mock.go", "mock", v1alpha1Pkg, "BeaconChainClient"},
				{mockPath + "/beacon_validator_server_mock.go", "mock", v1alpha1Pkg, "BeaconNodeValidatorServer,BeaconNodeValidator_WaitForActivationServer,BeaconNodeValidator_WaitForChainStartServer"},
				{mockPath + "/beacon_validator_client_mock.go", "mock", v1alpha1Pkg, "BeaconNodeValidatorClient,BeaconNodeValidator_WaitForChainStartClient,BeaconNodeValidator_WaitForActivationClient"},
				{mockPath + "/node_service_mock.go", "mock", v1alpha1Pkg, "NodeClient"},
				{mockPath + "/event_service_mock.go", "mock", servicePkg, "EventsClient,Events_StreamEventsClient,Events_StreamEventsServer"},
				{mockPath + "/keymanager_mock.go", "mock", validatorClientPkg, "RemoteSignerClient"},
			},
			format: true,
		},
		{
			dir: ifaceMockPath,
			reflect: []reflectMock{
				{ifaceMockPath + "/beacon_chain_client_mock.go", "validator_mock", ifacePkg, "BeaconChainClient"},
				{ifaceMockPath + "/node_client_mock.go", "validator_mock", ifacePkg, "NodeClient"},
				{ifaceMockPath + "/validator_client_mock.go", "validator_mock", ifacePkg, "ValidatorClient"},
			},
		},
		{
			dir: beaconAPIMockPath,
			source: []sourceMock{
				{beaconAPIMockPath + "/genesis_mock.go", "mock", "validator/client/beacon-api/genesis.go"},
				{beaconAPIMockPath + "/duties_mock.go", "mock", "validator/client/beacon-api/duties.go"},
				{beaconAPIMockPath + "/json_rest_handler_mock.go", "mock", "validator/client/beacon-api/json_rest_handler.go"},
				{beaconAPIMockPath + "/state_validators_mock.go", "mock", "validator/client/beacon-api/state_validators.go"},
				{beaconAPIMockPath + "/beacon_block_converter_mock.go", "mock", "validator/client/beacon-api/beacon_block_converter.go"},
			},
			format: true,
		},
		{
			dir: mlDSA87MockPath,
			source: []sourceMock{
				{mlDSA87MockPath + "/interface_mock.go", "mock", "crypto/ml_dsa_87/common/interface.go"},
			},
		},
	}
}

func genMocks() error {
	for _, g := range mockGroups() {
		for _, m := range g.reflect {
			fmt.Printf("generating %s for interfaces: %s\n", m.dest, m.interfaces)
			if err := mockgen("-package="+m.pkg, "-destination="+m.dest, m.importPath, m.interfaces); err != nil {
				return fmt.Errorf("mockgen: %w", err)
			}
		}

		for _, m := range g.source {
			fmt.Printf("generating %s for file: %s\n", m.dest, m.source)
			if err := mockgen("-package="+m.pkg, "-source="+m.source, "-destination="+m.dest); err != nil {
				return fmt.Errorf("mockgen: %w", err)
			}
		}

		// mockgen v1.6.0 predates `any`; the checked-in mocks use it.
		if err := sh("gofmt", append([]string{"-r", "interface{} -> any", "-w"}, g.dests()...)...); err != nil {
			return fmt.Errorf("gofmt -r: %w", err)
		}

		if !g.format {
			continue
		}

		if err := goimports(g.dir + "/."); err != nil {
			return fmt.Errorf("goimports: %w", err)
		}

		if err := gofmtSimplify(g.dir + "/."); err != nil {
			return fmt.Errorf("gofmtSimplify: %w", err)
		}
	}

	return nil
}
