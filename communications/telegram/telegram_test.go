package telegram

import (
	"testing"

	"github.com/yurulab/gocryptotrader/communications/base"
	"github.com/yurulab/gocryptotrader/config"
)

const (
	testErrNotFound = "Not Found"
)

var T Telegram

func TestSetup(t *testing.T) {
	cfg := config.GetConfig()
	err := cfg.LoadConfig("../../testdata/configtest.json", true)
	if err != nil {
		t.Fatal(err)
	}
	commsCfg := cfg.GetCommunicationsConfig()
	T.Setup(&commsCfg)
	if T.Name != "Telegram" || T.Enabled || T.Token != "testest" || T.Verbose {
		t.Error("telegram Setup() error, unexpected setup values",
			T.Name,
			T.Enabled,
			T.Token,
			T.Verbose)
	}
}

func TestConnect(t *testing.T) {
	err := T.Connect()
	if err == nil {
		t.Error("telegram Connect() error")
	}
}

func TestPushEvent(t *testing.T) {
	err := T.PushEvent(base.Event{})
	if err != nil {
		t.Error("telegram PushEvent() error", err)
	}
	T.AuthorisedClients = append(T.AuthorisedClients, 1337)
	err = T.PushEvent(base.Event{})
	if err.Error() != testErrNotFound {
		t.Errorf("telegram PushEvent() error, expected 'Not found' got '%s'",
			err)
	}
}

func TestHandleMessages(t *testing.T) {
	t.Parallel()
	chatID := int64(1337)
	err := T.HandleMessages(cmdHelp, chatID)
	if err.Error() != testErrNotFound {
		t.Errorf("telegram HandleMessages() error, expected 'Not found' got '%s'",
			err)
	}
	err = T.HandleMessages(cmdStart, chatID)
	if err.Error() != testErrNotFound {
		t.Errorf("telegram HandleMessages() error, expected 'Not found' got '%s'",
			err)
	}
	err = T.HandleMessages(cmdStatus, chatID)
	if err.Error() != testErrNotFound {
		t.Errorf("telegram HandleMessages() error, expected 'Not found' got '%s'",
			err)
	}
	err = T.HandleMessages(cmdSettings, chatID)
	if err.Error() != testErrNotFound {
		t.Errorf("telegram HandleMessages() error, expected 'Not found' got '%s'",
			err)
	}
	err = T.HandleMessages("Not a command", chatID)
	if err.Error() != testErrNotFound {
		t.Errorf("telegram HandleMessages() error, expected 'Not found' got '%s'",
			err)
	}
}

func TestGetUpdates(t *testing.T) {
	t.Parallel()
	_, err := T.GetUpdates()
	if err != nil {
		t.Error("telegram GetUpdates() error", err)
	}
}

func TestTestConnection(t *testing.T) {
	t.Parallel()
	err := T.TestConnection()
	if err.Error() != testErrNotFound {
		t.Errorf("telegram TestConnection() error, expected 'Not found' got '%s'",
			err)
	}
}

func TestSendMessage(t *testing.T) {
	t.Parallel()
	err := T.SendMessage("Test message", int64(1337))
	if err.Error() != testErrNotFound {
		t.Errorf("telegram SendMessage() error, expected 'Not found' got '%s'",
			err)
	}
}

func TestSendHTTPRequest(t *testing.T) {
	t.Parallel()
	err := T.SendHTTPRequest("0.0.0.0", nil, nil)
	if err == nil {
		t.Error("telegram SendHTTPRequest() error")
	}
}
