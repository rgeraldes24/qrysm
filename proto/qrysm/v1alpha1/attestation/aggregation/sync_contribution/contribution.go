package sync_contribution

import (
	"github.com/sirupsen/logrus"
	v1alpha1 "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

var _ = logrus.WithField("prefix", "aggregation.sync_contribution")

// Aggregate aggregates sync contributions. The minimal number of sync contributions is returned.
// Aggregation occurs in-place i.e. contents of input array will be modified. Should you need to
// preserve input sync contributions, clone them before aggregating.
func Aggregate(cs []*v1alpha1.SyncCommitteeContribution) ([]*v1alpha1.SyncCommitteeContribution, error) {
	return naiveSyncContributionAggregation(cs)
}
