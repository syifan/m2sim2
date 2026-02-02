package latency_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLatency(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Latency Suite")
}
