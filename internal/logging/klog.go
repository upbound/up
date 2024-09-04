// Copyright 2024 Upbound Inc.
// All rights reserved

package logging

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
)

type KlogFilter func(msg string, keysAndValues ...interface{}) bool

// SetFilteredKlogLogger sets log as the logger backend of klog, with debugLevel+3 as the
// klog verbosity level. If debugLevel is 0, only request throttling messages are
// logged. Further filters can be added to the logger by passing them as arguments.
// Those filters also only apply for debugLevel 0.
func SetFilteredKlogLogger(debugLevel int, log logr.Logger, preds ...KlogFilter) {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(fs)
	_ = fs.Parse([]string{fmt.Sprintf("--v=%d", debugLevel+3)}) //nolint:errcheck // we couldn't do anything here anyway

	preds = append(preds, requestThrottlingFilter)
	if debugLevel == 0 {
		preds = nil
	}

	klogr := logr.New(&klogFilter{LogSink: log.GetSink(), preds: preds})
	klog.SetLogger(klogr)
}

type klogFilter struct {
	logr.LogSink
	preds []KlogFilter
}

func (l *klogFilter) Info(level int, msg string, keysAndValues ...interface{}) {
	if len(l.preds) == 0 {
		l.LogSink.Info(klogToLogrLevel(level), msg, keysAndValues...)
		return
	}
	for _, pred := range l.preds {
		if pred(msg, keysAndValues...) {
			l.LogSink.Info(klogToLogrLevel(level), msg, keysAndValues...)
			return
		}
	}
}

func (l *klogFilter) Enabled(_ int) bool {
	return true
}

func klogToLogrLevel(klogLvl int) int {
	if klogLvl > 3 {
		return 1
	}
	return 0
}

func (l *klogFilter) WithCallDepth(depth int) logr.LogSink {
	if delegate, ok := l.LogSink.(logr.CallDepthLogSink); ok {
		return &klogFilter{LogSink: delegate.WithCallDepth(depth), preds: l.preds}
	}

	return l
}

// requestThrottlingFilter drops everything that is not a client-go throttling
// message, compare:
// https://github.com/kubernetes/client-go/blob/8c4efe8d079e405329f314fb789a41ac6af101dc/rest/request.go#L621
func requestThrottlingFilter(msg string, _ ...interface{}) bool {
	return strings.Contains(msg, "Waited for ") && strings.Contains(msg, "  request: ")
}
