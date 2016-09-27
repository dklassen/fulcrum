package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
)

type Checkpoint struct {
	FilePath             string
	LastSeenID           string
	HasReachedCheckpoint bool
}

func NewCheckpoint(prefix string) *Checkpoint {
	fp := fmt.Sprintf("/tmp/%s_candidate_id", prefix)
	logrus.Info("creating new checkpoint file", fp)
	return &Checkpoint{FilePath: fp, HasReachedCheckpoint: false}
}

func (cp *Checkpoint) ReachedCheckpoint(id string) bool {
	lastID := cp.LastProcessedID()

	if strings.Compare(lastID, "") == 0 {
		cp.LastSeenID = id
		cp.HasReachedCheckpoint = true
	}

	if !cp.HasReachedCheckpoint && strings.Compare(id, cp.LastProcessedID()) == 0 {
		cp.HasReachedCheckpoint = true
	}

	return cp.HasReachedCheckpoint
}

func (cp *Checkpoint) LastProcessedID() string {
	if strings.Compare(cp.LastSeenID, "") != 0 {
		return cp.LastSeenID
	}

	var lastID []byte
	var err error
	if lastID, err = ioutil.ReadFile(cp.FilePath); err != nil {
		logrus.Error(err)
	}

	cp.LastSeenID = string(lastID)
	return cp.LastSeenID
}

func (cp *Checkpoint) UpdateLastID(id string) {
	cp.LastSeenID = id
}

func (cp *Checkpoint) CheckPoint() {
	logrus.Info("Checkpointing ", cp.LastProcessedID())
	if err := ioutil.WriteFile(cp.FilePath, []byte(cp.LastProcessedID()), 0644); err != nil {
		logrus.Fatal(err)
	}
}

func (cp *Checkpoint) Remove() {
	os.Remove(cp.FilePath)
}
