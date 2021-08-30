package smtpservice

import (
	"testing"

	"github.com/yurulab/gocryptotrader/communications/base"
	"github.com/yurulab/gocryptotrader/config"
)

var s SMTPservice

func TestSetup(t *testing.T) {
	cfg := config.GetConfig()
	err := cfg.LoadConfig("../../testdata/configtest.json", true)
	if err != nil {
		t.Fatal(err)
	}
	commsCfg := cfg.GetCommunicationsConfig()
	s.Setup(&commsCfg)
}

func TestConnect(t *testing.T) {
	err := s.Connect()
	if err != nil {
		t.Error("smtpservice Connect() error", err)
	}
}

func TestPushEvent(t *testing.T) {
	err := s.PushEvent(base.Event{})
	if err == nil {
		t.Error("smtpservice PushEvent() error cannot be nil")
	}
}

func TestSend(t *testing.T) {
	err := s.Send("", "")
	if err == nil {
		t.Error("smtpservice Send() error cannot be nil")
	}
	err = s.Send("subject", "alertmessage")
	if err == nil {
		t.Error("smtpservice Send() error cannot be nil")
	}
}
