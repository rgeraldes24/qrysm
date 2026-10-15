package remote_web3signer

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	signRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_sign_requests_total",
		Help: "Total number of sign requests",
	})
	erroredResponsesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_errored_responses_total",
		Help: "Total number of errored responses when calling web3signer",
	})
	signRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "remote_web3signer_sign_request_duration_seconds",
			Help:    "Time (in seconds) from sending a sign request to having a verified signature, by artifact type",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type"},
	)
	randaoRevealRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_randao_reveal_requests_total",
		Help: "Total number of randao reveal requests",
	})
	aggregationSlotSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_aggregation_slot_requests_total",
		Help: "Total number of aggregation slot requests",
	})
	aggregateAndProofSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_aggregate_and_proof_sign_requests_total",
		Help: "Total number of aggregate and proof sign requests",
	})
	attestationSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_attestation_sign_requests_total",
		Help: "Total number of attestation sign requests",
	})
	blockZondSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_block_zond_sign_requests_total",
		Help: "Total number of block zond sign requests",
	})
	blindedBlockZondSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_blinded_block_zond_sign_requests_total",
		Help: "Total number of blinded block zond sign requests",
	})
	voluntaryExitSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_voluntary_exit_sign_requests_total",
		Help: "Total number of voluntary exit sign requests",
	})
	syncCommitteeMessageSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_sync_committee_message_sign_requests_total",
		Help: "Total number of sync committee message sign requests",
	})
	syncCommitteeSelectionProofSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_sync_committee_selection_proof_sign_requests_total",
		Help: "Total number of sync committee selection proof sign requests",
	})
	syncCommitteeContributionAndProofSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_sync_committee_contribution_and_proof_sign_requests_total",
		Help: "Total number of sync committee contribution and proof sign requests",
	})
	validatorRegistrationSignRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "remote_web3signer_validator_registration_sign_requests_total",
		Help: "Total number of validator registration sign requests",
	})
)
