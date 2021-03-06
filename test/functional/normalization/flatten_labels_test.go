package normalization

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	logging "github.com/openshift/cluster-logging-operator/pkg/apis/logging/v1"
	"github.com/openshift/cluster-logging-operator/test/functional"
	"github.com/openshift/cluster-logging-operator/test/helpers"
)

var _ = Describe("[Normalization] Fluentd normalization", func() {

	const (
		sampleMetric = "# HELP fluentd_output_status_buffer_total_bytes"
	)

	var (
		framework *functional.FluentdFunctionalFramework
	)

	BeforeEach(func() {
		framework = functional.NewFluentdFunctionalFramework()
		functional.NewClusterLogForwarderBuilder(framework.Forwarder).
			FromInput(logging.InputNameApplication).
			ToFluentForwardOutput()
		Expect(framework.Deploy()).To(BeNil())
		Expect(framework.WritesApplicationLogs(1)).To(BeNil())
	})
	AfterEach(func() {
		framework.Cleanup()
	})

	It("should remove 'kubernetes.labels' and create 'kubernetes.flat_labels' with an array of 'kubernetes.labels'", func() {
		raw, err := framework.ReadApplicationLogsFrom("fluentforward")
		Expect(err).To(BeNil(), "Expected no errors reading the logs")
		logs, err := helpers.ParseLogs(raw)
		Expect(err).To(BeNil(), "Expected no errors parsing the logs")
		//verify the new key exists
		Expect(logs[0].Kubernetes.FlatLabels).To(Not(BeNil()), fmt.Sprintf("Expected to find the kubernetes.flat_labels key in %#v", logs[0]))

		//verify we removed the old key
		Expect(logs[0].Kubernetes.Labels).To(BeNil(), fmt.Sprintf("Did not expect to find the kubernetes.labels key in %#v", logs[0]))
	})

})
