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
	"github.com/bradfitz/gomemcache/memcache"
	"log"
	"os"
	"syscall"
	"time"
	"fmt"
	"strings"
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
	fmt.Fprintf(os.Stderr, "%s: Distributed mutex using memcache.\n\n" +
		"Given the same script on multiple servers, %s ensures only one\n" +
		"will run at a given time. Particularly useful with cronjobs.\n\n" +
		"Usage example:\n\n" +
		"  $ %s /bin/date -u\n\n" +
		"Documentation and source code at: http://github.com/realgeeks/sherlock\n\n" +
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
		out, err := os.OpenFile(Logfile(), os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0666)
		if err != nil {
			panic(fmt.Sprintf("Cannot open log file %s: %s", Logfile(), err))
		}
		log.SetOutput(out)
		defer out.Close()
	}

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

	proc, err := os.StartProcess(args[0], args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		log.Panicf("Failed to start process: %s", err)
	}

	state, err := proc.Wait()
	if err != nil {
		log.Panicf("Could not determine exit status of process: %s", err)
	}
	status := state.Sys().(syscall.WaitStatus)
	return status.ExitStatus()
}

func main() {
	os.Exit(run())
}
