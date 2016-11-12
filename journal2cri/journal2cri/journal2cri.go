/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package journal2cri contains functions to convert a systemd journal to a CRI logfile
package journal2cri

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
)

// ProcessEntry parses a journal entry into a struct containing the info needed for CRI logging
func ProcessEntry(entry *sdjournal.JournalEntry) *CRIEntry {
	rktAppName, ok := entry.Fields["SYSLOG_IDENTIFIER"]
	if !ok {
		// Not an app entry
		return nil
	}

	// App names have the special format of 'appname-instanceNumber' by convention
	parts := strings.SplitN(rktAppName, "-", 2)
	if len(parts) != 2 {
		return nil
	}

	appNumberStr := parts[0]
	if appNumberStr == "rktletinternal" {
		return nil
	}
	appName := parts[1]

	appNumber, err := strconv.Atoi(appNumberStr)
	if err != nil {
		log.Printf("could not parse apps attempt: %v in %v, %v", appNumberStr, rktAppName, err)
		return nil
	}
	appName = strings.TrimLeft(appName, "-")
	if len(appName) == 0 {
		log.Printf("unexpected 0 length appName in %v", rktAppName)
		return nil
	}

	// BUG(euank): this doesn't actually get set to stderr ever; journald does not distingish this how we use it
	outStream := entry.Fields["_TRANSPORT"]
	if outStream != string(CRIStreamStdout) && outStream != string(CRIStreamStderr) {
		log.Printf("unrecognized out stream type: %v", outStream)
		return nil
	}

	return &CRIEntry{
		AppName:    appName,
		AppAttempt: appNumber,
		Message:    entry.Fields["MESSAGE"],
		StreamType: CRIStreamType(outStream),
		Timestamp:  time.Unix(0, int64(time.Duration(entry.RealtimeTimestamp)*time.Microsecond)),
	}
}

type CRIStreamType string

const (
	CRIStreamStdout CRIStreamType = "stdout"
	CRIStreamStderr               = "stderr"
)

type CRIEntry struct {
	AppName    string
	AppAttempt int
	Message    string
	StreamType CRIStreamType
	Timestamp  time.Time
}

// WriteEntry writes a CRI entry to a file at the expected location
// TODO we really should be holding onto file ptrs, this constant reopen/closing is not good
func WriteEntry(entry *CRIEntry, dir string) {
	fileName := fmt.Sprintf("%s_%d.log", entry.AppName, entry.AppAttempt)
	path := filepath.Join(dir, fileName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Printf("could not open file for append: %v", err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("%s %s %s\n", entry.Timestamp.Format(time.RFC3339Nano), entry.StreamType, entry.Message))
	if err != nil {
		log.Printf("could not append file: %v", err)
	}
}
