package cmd_test

import (
	"context"
	"io"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/cluster-logging-operator/test"
	. "github.com/openshift/cluster-logging-operator/test/helpers/cmd"
	. "github.com/openshift/cluster-logging-operator/test/matchers"
)

var _ = Describe("CmdReader", func() {
	It("reads lines", func() {
		cmd := exec.Command("echo", "a\nb\nc\n")
		r, err := NewReader(cmd)
		ExpectOK(err)
		defer r.Close()
		for _, s := range []string{"a\n", "b\n", "c\n"} {
			Expect(r.ReadLine()).To(Equal(s))
		}
	})

	It("times out", func() {
		cmd := exec.Command("sleep", "1m")
		r, err := NewReader(cmd)
		ExpectOK(err)
		defer r.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second/100)
		defer cancel()
		_, err = r.ReadLineContext(ctx)
		Expect(err).To(HaveOccurred())
	})

	It("ExpectEmpty passes on EOF", func() {
		r, err := NewReader(exec.Command("true"))
		ExpectOK(err)
		defer r.Close()
		ExpectOK(r.ExpectEmpty(context.Background()))
	})

	It("ExpectEmpty passes on timeout", func() {
		r, err := NewReader(exec.Command("sleep", "1m"))
		ExpectOK(err)
		defer r.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second/10)
		defer cancel()
		ExpectOK(r.ExpectEmpty(ctx))
	})

	It("ExpectEmpty fails", func() {
		r, err := NewReader(exec.Command("echo", "hello\n"))
		ExpectOK(err)
		defer r.Close()
		ctx, cancel := context.WithTimeout(context.Background(), test.SuccessTimeout())
		defer cancel()
		Expect(r.ExpectEmpty(ctx)).To(MatchError(`expected empty, read line: "hello\n"`))
	})

	It("ExpectLines passes", func() {
		r, err := NewReader(exec.Command("echo", "hello\nignoreme\nhello there\n"))
		ExpectOK(err)
		defer r.Close()
		ExpectOK(r.ExpectLines(2, "hello", ""))
	})

	It("ExpectLines fails with bad lines", func() {
		r, err := NewReader(exec.Command("echo", "hello\nwho's bad?\nhello there\n"))
		ExpectOK(err)
		defer r.Close()
		Expect(r.ExpectLines(2, "hello", "bad")).To(MatchError(`bad line: "who's bad?\n"`))
	})

	It("ExpectLines fails with not enough lines", func() {
		r, err := NewReader(exec.Command("echo", "hello\nignoreme\n"))
		ExpectOK(err)
		defer r.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second/10)
		defer cancel()
		Expect(r.ExpectLinesContext(ctx, 2, "hello", "bad")).To(MatchError(io.EOF))
	})
})
