package loginsight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLoginsight(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Loginsight Suite")
}
