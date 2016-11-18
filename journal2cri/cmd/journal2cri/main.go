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

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/kubernetes-incubator/rktlet/journal2cri/journal2cri"
)

func usage() {
	log.Println(fmt.Sprintf("Usage: %s <path-to-pod-journal-dir> <path-to-pod-logdir>", os.Args[0]))
	os.Exit(1)
}

func main() {
	cmdArgs := os.Args[1:]
	if len(cmdArgs) != 2 {
		usage()
	}

	journalDir := cmdArgs[0]
	criDir := cmdArgs[1]

	// The information we need to log is the container name, the container "instance number", and the actual data.
	// Fortunately for us, all of that information can be retrieved by parsing the app name. So we just do that.
	journal, err := sdjournal.NewJournalFromDir(journalDir)
	if err != nil {
		log.Fatalf("could not load journal from dir: %v", err)
	}

	err = journal.SeekHead()
	if err != nil {
		log.Fatalf("could not seek head: %v", err)
	}

	// Keep track of the last cursor we've processed so we know when the journal
	// hasn't advanced any
	lastCursor := ""
	for {
		_, err := journal.Next()
		if err != nil {
			log.Printf("error getting next journal entry: %v", err)
			continue
		}

		entry, err := journal.GetEntry()
		if err != nil {
			log.Printf("error getting journal entry: %v", err)
			continue
		}

		if lastCursor == entry.Cursor {
			// Next didn't actually advance us, wait for new data
			journal.Wait(1 * time.Second)
			continue
		}

		// New entry, process it
		criEntry := journal2cri.ProcessEntry(entry)
		if criEntry != nil {
			journal2cri.WriteEntry(criEntry, criDir)
		}
		lastCursor = entry.Cursor
	}
}
