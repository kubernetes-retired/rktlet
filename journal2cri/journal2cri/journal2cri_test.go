package journal2cri

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/stretchr/testify/assert"
)

func TestProcessEntry(t *testing.T) {
	now := time.Unix(0xbad, 0xfacade)
	now = now.Round(time.Microsecond) // since the nanosecond part of now will break assertions
	timeInMicros := now.UnixNano() / 1000
	successPairs := []struct {
		In  sdjournal.JournalEntry
		Out *CRIEntry
	}{
		{
			In: sdjournal.JournalEntry{
				RealtimeTimestamp: uint64(timeInMicros),
				Fields: map[string]string{
					"SYSLOG_IDENTIFIER": "1-myapp",
					"_TRANSPORT":        "stdout",
					"MESSAGE":           "20/20",
				},
			},
			Out: &CRIEntry{
				AppName:    "myapp",
				AppAttempt: 1,
				Message:    "20/20",
				StreamType: CRIStreamStdout,
				Timestamp:  now,
			},
		},
		{
			In: sdjournal.JournalEntry{
				RealtimeTimestamp: uint64(timeInMicros),
				Fields: map[string]string{
					"SYSLOG_IDENTIFIER": "10-otherapp",
					"_TRANSPORT":        "stderr",
					"MESSAGE":           "petrov",
				},
			},
			Out: &CRIEntry{
				AppName:    "otherapp",
				AppAttempt: 10,
				Message:    "petrov",
				StreamType: CRIStreamStderr,
				Timestamp:  now,
			},
		},
		{
			In: sdjournal.JournalEntry{
				RealtimeTimestamp: uint64(timeInMicros),
				Fields: map[string]string{
					"SYSLOG_IDENTIFIER": "rktletinternal-journal2cri",
					"_TRANSPORT":        "stdout",
					"MESSAGE":           "ghost",
				},
			},
			Out: nil,
		},
	}

	for i, pair := range successPairs {
		appName := "nil"
		if pair.Out != nil {
			appName = pair.Out.AppName
		}
		t.Run(fmt.Sprintf("%d success with %s", i, appName), func(t *testing.T) {
			out := ProcessEntry(&pair.In)
			assert.Equal(t, pair.Out, out)
		})
	}
}
