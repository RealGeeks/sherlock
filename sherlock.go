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
	"errors"
	"flag"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

var options = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

var (
	once    = options.Bool("once", false, "Do not run the program if lock is acquired by somebody else")
	key     = options.String("memcache-key", "mutex-default", "Key to be used as lock in memcache")
	servers = options.String("memcache-servers", "127.0.0.1:11211", "Comma separared list of memcache servers")
	verbose = options.Bool("verbose", false, "More verbose output")
	logfile = options.String("logfile", "stdout", "File to write log messages.")
)

func init() {
	options.Usage = Usage
}

func Usage() {
	fmt.Fprintf(os.Stderr, "%s: Distributed mutex using memcache.\n\n"+
		"Given the same script on multiple servers, %s ensures only one\n"+
		"will run at a given time. Particularly useful with cronjobs.\n\n"+
		"Usage example:\n\n"+
		"  $ %s /bin/date -u\n\n"+
		"Documentation and source code at: http://github.com/realgeeks/sherlock\n\n"+
		"Options:\n",
		os.Args[0], os.Args[0], os.Args[0])
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

func run() int {
	options.Parse(os.Args[1:])

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

	args := options.Args()
	if len(args) == 0 {
		log.Fatal("No program specified. See -help.")
	}
	log.Printf("Running %v", args)

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

	signals := watchSignals()
	child, exit, errs := startProcess(args)

	for {
		select {
		case status := <-exit:
			Debugf("Program existed with status code: %d", status)
			return status
		case err = <-errs:
			log.Printf("Failed to fork/exec/wait on process: %s", err)
			return 25
		case sig := <-signals:
			Debugf("Received signal: %v. Forwarding to process", sig)
			child.Signal(sig)
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

// Starts a new process and wait()s for it to finish in a separate goroutine.
//
// The new process is added to a new group, different from the parent; this
// avoids the behavior of pressing CTRL+C on a sherlock process and the SIGINT
// being sent to sherlock's childs (because they are in the same group by default)
//
// Returns the child *os.Process; a channel that will receive the exit
// status once the process is finished; and a channel that will receive
// any error when creating or wait()ing on the process
func startProcess(args []string) (child *os.Process, exit chan int, errs chan error) {
	exit = make(chan int)
	errs = make(chan error, 1)
	child, err := os.StartProcess(
		args[0], args,
		&os.ProcAttr{
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
			Sys:   &syscall.SysProcAttr{Setpgid: true},
		},
	)
	if err != nil {
		errs <- err
		return child, exit, errs
	}
	go func() {
		state, err := child.Wait()
		if err != nil {
			errs <- err
			return
		}
		status := state.Sys().(syscall.WaitStatus)
		exit <- status.ExitStatus()
	}()
	return child, exit, errs
}

func main() {
	os.Exit(run())
}
