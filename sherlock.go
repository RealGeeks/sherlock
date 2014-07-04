// Copyright (c) 2013 Igor Sobreira igor@igorsobreira.com
//
// Permission is hereby granted, free of charge, to any person
// obtaining a copy of this software and associated documentation
// files (the "Software"), to deal in the Software without
// restriction, including without limitation the rights to use,
// copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
// OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
// HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
// WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE
// OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

var options = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

const Version = "1.1"

var (
	once    = options.Bool("once", false, "Do not run the program if lock is acquired by somebody else")
	key     = options.String("memcache-key", "mutex-default", "Key to be used as lock in memcache")
	servers = options.String("memcache-servers", "127.0.0.1:11211", "Comma separared list of memcache servers")
	verbose = options.Bool("verbose", false, "More verbose output")
	logfile = options.String("logfile", "stdout", "File to write log messages. Program stdout and stderr will be written here too")
	showver = options.Bool("version", false, "Show version")
)

// exit status when failed to run subprocess
const errStatus = 25

func init() {
	options.Usage = Usage
}

func Usage() {
	fmt.Fprintf(os.Stderr, "%s %s: Distributed mutex using memcache.\n\n"+
		"Given the same script on multiple servers, %s ensures only one\n"+
		"will run at a given time. Particularly useful with cronjobs.\n\n"+
		"Usage example:\n\n"+
		"  $ %s -verbose /bin/date -u\n\n"+
		"Note that -verbose is a sherlock flag while -u is given to /bin/date\n\n"+
		"Documentation and source code at: http://github.com/realgeeks/sherlock\n\n"+
		"Options:\n",
		os.Args[0], Version, os.Args[0], os.Args[0])
	options.PrintDefaults()
}

func Retry() bool {
	return *once == false
}

func Key() string {
	return *key
}

func Servers() []string {
	return strings.Split(*servers, ",")
}

func Logfile() string {
	return *logfile
}

func ShowVersion() bool {
	return *showver
}

func Debug(v ...interface{}) {
	if *verbose {
		log.Print(v...)
	}
}

func Debugf(fmt string, v ...interface{}) {
	if *verbose {
		log.Printf(fmt, v...)
	}
}

var (
	ErrDuplicateAcquire = errors.New("Acquired by somebody else and retry disabled with -once")
)

type MemcLock struct {
	memc *memcache.Client
}

func NewMemcLock() *MemcLock {
	return &MemcLock{memc: memcache.New(Servers()...)}
}

func (ml *MemcLock) Acquire() error {
	Debug("Acquiring lock")
	for {
		err := ml.memc.Add(&memcache.Item{
			Key:        Key(),
			Value:      []byte{'H', 'I'},
			Expiration: 0,
		})
		if err == nil {
			Debug("Acquired")
			return nil
		}
		if err == memcache.ErrNotStored {
			if !Retry() {
				return ErrDuplicateAcquire
			} else {
				Debug("Retrying")
				time.Sleep(100 * time.Millisecond)
			}
		} else {
			return err
		}
	}
}

func (ml *MemcLock) Release() {
	Debug("Releasing")
	ml.memc.Delete(Key())
}

// run executes the process and manages it.
//
// args must be the same format as os.Args.
func run(args []string) int {

	// parse sherlock flags, don't give program name (sherlock).
	// the real program to execute should be after all sherlock flags
	// and will accessible as options.Args()
	options.Parse(args[1:])

	if ShowVersion() {
		fmt.Fprintf(os.Stderr, "%s %s\n", os.Args[0], Version)
		return 0
	}

	if Logfile() != "stdout" {
		out, err := os.OpenFile(Logfile(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic(fmt.Sprintf("Cannot open log file %s: %s", Logfile(), err))
		}
		log.SetOutput(out)
		defer out.Close()
	}

	// has to be after log setup otherwise logfile will be closed
	defer func() {
		if e := recover(); e != nil {
			log.Printf("Recovered from panic: %s\n%s", e, debug.Stack())
		}
	}()

	// options.Args() are the arguments after all sherlock flags are
	// parsed, which means: the program to run
	programArgs := options.Args()
	if len(programArgs) == 0 {
		log.Fatal("No program specified. See -help.")
	}
	log.Printf("Running %v", programArgs)

	mutex := NewMemcLock()
	err := mutex.Acquire()
	if err != nil {
		if err == ErrDuplicateAcquire {
			log.Print(err)
			return 0
		}
		log.Panicf("Failed to acquire lock: %s", err)
	}
	defer mutex.Release()

	// sherlock will listen to some signals and forward them
	// to underlying process
	signals := watchSignals()

	proc, err := newProcess(programArgs)
	if err != nil {
		log.Printf("Failed to start process: %s", err)
		return errStatus
	}

	for {
		select {
		case sig := <-signals:
			Debugf("Received signal: %v. Forwarding to process", sig)
			proc.Signal(sig)
		case <-proc.Wait():
			if proc.err != nil {
				log.Printf("Process execution failed: %s", proc.err)
			}
			log.Printf("Program stdout:\n%s", proc.stdout)
			log.Printf("Program stderr:\n%s", proc.stderr)
			Debugf("Program exited with status code: %d", proc.status)
			return proc.status
		}
	}
	return 0 // will not happen
}

// Starts listening to signals that should quit the process.
//
// Returns a channel where received signals will be published
func watchSignals() (sinals chan os.Signal) {
	// Use a buffered channel so we don't miss a signal sent to it
	// before we start listening (package signal will not block
	// sending on this channel)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals,
		syscall.SIGQUIT,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	return signals
}

// process wraps a exec.Cmd execution and keeps it's exit
// status, stdout and stderr
//
// err will be populated with an error if Wait() fails
type process struct {
	cmd            *exec.Cmd
	status         int
	err            error
	stdout, stderr io.Reader
	finished       chan struct{}
}

// newProcess creates a new process, starts it and wait for it to finish
// is a separate goroutine
//
// Returns error process fails to start
//
// Process is added to a new group. If CTRL+C is pressed in a shell it sends
// SIGINT to all process in that group, and the subprocess is added to the
// same group by default. I don't want that, I want sherlock to send termination
// signals to it's subprocess.
func newProcess(args []string) (*process, error) {
	var out, err bytes.Buffer

	proc := &process{
		cmd:      exec.Command(args[0], args[1:]...),
		stdout:   &out, // process uses as io.Reader
		stderr:   &err,
		finished: make(chan struct{}, 1),
	}
	proc.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // add process to new group, different than sherlock's
	}
	proc.cmd.Stdout = &out // cmd uses as io.Writer
	proc.cmd.Stderr = &err

	if err := proc.cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		if err := proc.cmd.Wait(); err != nil {
			proc.err = err
		}
		proc.storeStatus()
		proc.done()
	}()

	return proc, nil
}

// Signal sends a signal to the process
func (p *process) Signal(sig os.Signal) error {
	return p.cmd.Process.Signal(sig)
}

// Wait returns a channel where caller should receive from
// that indicates when the process has finished
//
// It returns a channel instead of just block to be used in
// a select
func (p *process) Wait() <-chan struct{} {
	return p.finished
}

func (p *process) done() {
	p.finished <- struct{}{}
}

// storeStatus saves the exit status of process. Called when
// process has finished
func (p *process) storeStatus() {
	status := p.cmd.ProcessState.Sys().(syscall.WaitStatus)
	p.status = status.ExitStatus()
}

func main() {
	os.Exit(run(os.Args))
}
