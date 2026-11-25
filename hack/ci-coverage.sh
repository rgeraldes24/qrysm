#!/bin/bash

# Run coverage tests.
go test -tags develop -coverprofile=/tmp/cover.out -covermode=atomic ./... || true

# Download deepsource CLI
curl https://deepsource.io/cli | sh

# Upload to deepsource (requires DEEPSOURCE_DSN environment variable)
./bin/deepsource report --analyzer test-coverage --key go --value-file /tmp/cover.out

# Provide permission to execute script.
chmod +x ./hack/codecov.sh

# Upload to codecov (requires CODECOV_TOKEN environment variable)
./hack/codecov.sh -f /tmp/cover.out
