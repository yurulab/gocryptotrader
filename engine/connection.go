package engine

import (
	"errors"
	"sync/atomic"

	"github.com/yurulab/gocryptotrader/connchecker"
	"github.com/yurulab/gocryptotrader/log"
)

// connectionManager manages the connchecker
type connectionManager struct {
	started int32
	stopped int32
	conn    *connchecker.Checker
}

// Started returns if the connection manager has started
func (c *connectionManager) Started() bool {
	return atomic.LoadInt32(&c.started) == 1
}

// Start starts an instance of the connection manager
func (c *connectionManager) Start() error {
	if atomic.AddInt32(&c.started, 1) != 1 {
		return errors.New("connection manager already started")
	}

	log.Debugln(log.ConnectionMgr, "Connection manager starting...")
	var err error
	c.conn, err = connchecker.New(Bot.Config.ConnectionMonitor.DNSList,
		Bot.Config.ConnectionMonitor.PublicDomainList,
		Bot.Config.ConnectionMonitor.CheckInterval)
	if err != nil {
		atomic.CompareAndSwapInt32(&c.started, 1, 0)
		return err
	}

	log.Debugln(log.ConnectionMgr, "Connection manager started.")
	return nil
}

// Stop stops the connection manager
func (c *connectionManager) Stop() error {
	if atomic.LoadInt32(&c.started) == 0 {
		return errors.New("connection manager not started")
	}

	if atomic.AddInt32(&c.stopped, 1) != 1 {
		return errors.New("connection manager is already stopped")
	}

	log.Debugln(log.ConnectionMgr, "Connection manager shutting down...")
	c.conn.Shutdown()
	atomic.CompareAndSwapInt32(&c.stopped, 1, 0)
	atomic.CompareAndSwapInt32(&c.started, 1, 0)
	log.Debugln(log.ConnectionMgr, "Connection manager stopped.")
	return nil
}

// IsOnline returns if the connection manager is online
func (c *connectionManager) IsOnline() bool {
	if c.conn == nil {
		log.Warnln(log.ConnectionMgr, "Connection manager: IsOnline called but conn is nil")
		return false
	}

	return c.conn.IsConnected()
}
