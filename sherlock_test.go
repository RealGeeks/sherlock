package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
)

const (
	// this file is sherlock log file, I'll read and make sure sherlock is
	// saying what I want
	testlog = "/tmp/sherlock-test.log"

	// this file is written by the program sherlock runs, sherlock_test_helper.py,
	// so I can assert sherlock is actually running something
	testrun = "/tmp/sherlock-test.run"
)

func init() {
	checkMemcached()
	checkPython()
}

func setup() {
	os.Remove(testlog)
	os.Remove(testrun)
}

func TestRun(t *testing.T) {
	setup()

	runSherlock([]string{"touchfile:" + testrun})

	if _, err := os.Stat(testrun); os.IsNotExist(err) {
		t.Errorf("testrun file wasn't created, sherlock_test_helper.py didn't run!")
	}
}

func TestCaptureProcessOutput(t *testing.T) {
	setup()

	runSherlock([]string{"stdout:normal output", "stderr:error output"})

	output := readTestlog()
	assertContains(t, output, "Program stdout:\nnormal output")
	assertContains(t, output, "Program stderr:\nerror output")
}

func runSherlock(extra []string) {
	args := []string{"sherlock", "-logfile", testlog, "/usr/bin/python", "sherlock_test_helper.py"}
	args = append(args, extra...)
	run(args)
}

func readTestlog() string {
	out, err := ioutil.ReadFile(testlog)
	if err != nil {
		panic("failed to read testlog: " + err.Error())
	}
	return string(out)
}

func assertContains(t *testing.T, s, substr string) {
	if !strings.Contains(s, substr) {
		t.Errorf("Substring not found.\nSTRING:\n%s\nSUBSTRING:\n%s\n",
			s, substr)
	}
}

func checkMemcached() {
	cli := memcache.New("127.0.0.1:11211")
	err := cli.Add(&memcache.Item{
		Key:        "test",
		Value:      []byte{'t'},
		Expiration: 1,
	})
	if err != nil && err != memcache.ErrNotStored {
		fmt.Printf("Please start memcached (%s)\n", err)
		os.Exit(11)
	}
}

func checkPython() {
	if _, err := os.Stat("/usr/bin/python"); os.IsNotExist(err) {
		fmt.Println("Tests require /usr/bin/python")
		os.Exit(11)
	}
}
