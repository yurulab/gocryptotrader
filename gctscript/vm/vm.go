package vm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"io/ioutil"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/gofrs/uuid"
	"github.com/yurulab/gocryptotrader/common/crypto"
	scriptevent "github.com/yurulab/gocryptotrader/database/repository/script"
	"github.com/yurulab/gocryptotrader/gctscript/modules/loader"
	"github.com/yurulab/gocryptotrader/gctscript/wrappers/validator"
	"github.com/yurulab/gocryptotrader/log"
	"github.com/volatiletech/null"
)

// NewVM attempts to create a new Virtual Machine firstly from pool
func NewVM() (vm *VM) {
	newUUID, err := uuid.NewV4()
	if err != nil {
		log.Error(log.GCTScriptMgr, Error{
			Action: "New: UUID",
			Cause:  err,
		})
		return nil
	}

	if GCTScriptConfig.Verbose {
		log.Debugln(log.GCTScriptMgr, "New GCTScript VM created")
	}

	vm = &VM{
		ID:     newUUID,
		Script: pool.Get().(*tengo.Script),
	}
	return
}

// SetDefaultScriptOutput sets default output file for scripts
func SetDefaultScriptOutput() {
	loader.SetDefaultScriptOutput(filepath.Join(ScriptPath, "output"))
}

// Load parses and creates a new instance of tengo script vm
func (vm *VM) Load(file string) error {
	if vm == nil {
		return ErrNoVMLoaded
	}

	if !GCTScriptConfig.Enabled {
		return &Error{
			Action: "Load",
			Cause:  ErrScriptingDisabled,
		}
	}

	if filepath.Ext(file) != ".gct" {
		file += ".gct"
	}

	if GCTScriptConfig.Verbose {
		log.Debugf(log.GCTScriptMgr, "Loading script: %s ID: %v", vm.ShortName(), vm.ID)
	}

	code, err := ioutil.ReadFile(file)
	if err != nil {
		return &Error{
			Action: "Load: ReadFile",
			Script: file,
			Cause:  err,
		}
	}

	vm.File = file
	vm.Path = filepath.Dir(file)
	vm.Script = tengo.NewScript(code)
	scriptctx := vm.ShortName() + "-" + vm.ID.String()
	err = vm.Script.Add("ctx", scriptctx)
	if err != nil {
		return err
	}

	vm.Script.SetImports(loader.GetModuleMap())
	vm.Hash = vm.getHash()

	if GCTScriptConfig.AllowImports {
		if GCTScriptConfig.Verbose {
			log.Debugf(log.GCTScriptMgr, "File imports enabled for vm: %v", vm.ID)
		}
		vm.Script.EnableFileImport(true)
	}
	vm.event(StatusSuccess, TypeLoad)
	return nil
}

// Compile compiles to byte code loaded copy of vm script
func (vm *VM) Compile() (err error) {
	vm.Compiled = new(tengo.Compiled)
	vm.Compiled, err = vm.Script.Compile()
	return
}

// Run runs byte code
func (vm *VM) Run() (err error) {
	if GCTScriptConfig.Verbose {
		log.Debugf(log.GCTScriptMgr, "Running script: %s ID: %v", vm.ShortName(), vm.ID)
	}

	err = vm.Compiled.Run()
	if err != nil {
		vm.event(StatusFailure, TypeExecute)
		return Error{
			Action: "Run",
			Cause:  err,
		}
	}
	vm.event(StatusSuccess, TypeExecute)
	return
}

// RunCtx runs compiled byte code with context.Context support.
func (vm *VM) RunCtx() (err error) {
	if vm.ctx == nil {
		vm.ctx = context.Background()
	}

	ct, cancel := context.WithTimeout(vm.ctx, GCTScriptConfig.ScriptTimeout)
	defer cancel()

	if GCTScriptConfig.Verbose {
		log.Debugf(log.GCTScriptMgr, "Running script: %s ID: %v", vm.ShortName(), vm.ID)
	}

	err = vm.Compiled.RunContext(ct)
	if err != nil {
		vm.event(StatusFailure, TypeExecute)
		return Error{
			Action: "RunCtx",
			Cause:  err,
		}
	}
	vm.event(StatusSuccess, TypeExecute)
	return
}

// CompileAndRun Compile and Run script with support for task running
func (vm *VM) CompileAndRun() {
	if vm == nil {
		return
	}
	err := vm.Compile()
	if err != nil {
		log.Error(log.GCTScriptMgr, err)
		err = RemoveVM(vm.ID)
		if err != nil {
			log.Error(log.GCTScriptMgr, err)
		}
		return
	}

	err = vm.RunCtx()
	if err != nil {
		log.Error(log.GCTScriptMgr, err)
		err = RemoveVM(vm.ID)
		if err != nil {
			log.Error(log.GCTScriptMgr, err)
		}
		return
	}
	if vm.Compiled.Get("timer").String() != "" {
		vm.T, err = time.ParseDuration(vm.Compiled.Get("timer").String())
		if err != nil {
			log.Error(log.GCTScriptMgr, err)
			err = vm.Shutdown()
			if err != nil {
				log.Error(log.GCTScriptMgr, err)
			}
			return
		}
		if vm.T < time.Nanosecond {
			log.Error(log.GCTScriptMgr, "Repeat timer cannot be under 1 nano second")
			err = vm.Shutdown()
			if err != nil {
				log.Errorln(log.GCTScriptMgr, err)
			}
			return
		}
		vm.runner()
	} else {
		err = vm.Shutdown()
		if err != nil {
			log.Error(log.GCTScriptMgr, err)
		}
		return
	}
}

// Shutdown shuts down current VM
func (vm *VM) Shutdown() error {
	if vm == nil {
		return ErrNoVMLoaded
	}
	if vm.S != nil {
		close(vm.S)
	}
	if GCTScriptConfig.Verbose {
		log.Debugf(log.GCTScriptMgr, "Shutting down script: %s ID: %v", vm.ShortName(), vm.ID)
	}
	vm.Script = nil
	pool.Put(vm.Script)
	vm.event(StatusSuccess, TypeStop)
	return RemoveVM(vm.ID)
}

// Read contents of script back and create script event
func (vm *VM) Read() ([]byte, error) {
	vm.event(StatusSuccess, TypeRead)
	return vm.read()
}

// Read contents of script back
func (vm *VM) read() ([]byte, error) {
	if GCTScriptConfig.Verbose {
		log.Debugf(log.GCTScriptMgr, "Read script: %s ID: %v", vm.ShortName(), vm.ID)
	}
	return ioutil.ReadFile(vm.File)
}

// ShortName returns short (just filename.extension) of running script
func (vm *VM) ShortName() string {
	return filepath.Base(vm.File)
}

func (vm *VM) event(status, executionType string) {
	if validator.IsTestExecution.Load() == true {
		return
	}

	var data null.Bytes
	if executionType == TypeLoad {
		scriptData, err := vm.scriptData()
		if err != nil {
			log.Errorf(log.GCTScriptMgr, "Failed to retrieve scriptData: %v", err)
		}
		data.SetValid(scriptData)
	}
	scriptevent.Event(vm.getHash(), vm.ShortName(), vm.Path, data, executionType, status, time.Now())
}

func (vm *VM) scriptData() ([]byte, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, err := w.Create(vm.ShortName())
	if err != nil {
		return []byte{}, err
	}
	contents, err := vm.read()
	if err != nil {
		return []byte{}, err
	}
	_, err = f.Write(contents)
	if err != nil {
		return []byte{}, err
	}
	err = w.Close()
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func (vm *VM) getHash() string {
	if vm.Hash != "" {
		return vm.Hash
	}
	contents, err := vm.read()
	if err != nil {
		log.Errorln(log.GCTScriptMgr, err)
	}
	contents = append(contents, vm.ShortName()...)
	return hex.EncodeToString(crypto.GetSHA256(contents))
}

func (vmc *vmscount) add() {
	atomic.AddInt32((*int32)(vmc), 1)
}

func (vmc *vmscount) remove() {
	atomic.AddInt32((*int32)(vmc), -1)
}

// Len() returns current length vmscount
func (vmc *vmscount) Len() int32 {
	return atomic.LoadInt32((*int32)(vmc))
}
