// SPDX-License-Identifier: Elastic-2.0

/*
 * Copyright 2022 Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under
 * one or more contributor license agreements. Licensed under the Elastic
 * License 2.0; you may not use this file except in compliance with the Elastic
 * License 2.0.
 */

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// This is a JSON type printed by the test binaries (not by EventsTrace), it's
// used all over the place, so define it here to save space
type TestPidInfo struct {
	Tid  int64 `json:"tid"`
	Tgid int64 `json:"tgid"`
	Ppid int64 `json:"ppid"`
	Pgid int64 `json:"pgid"`
	Sid  int64 `json:"sid"`
}

// Definitions of types printed by EventsTrace for conversion from JSON
type InitMsg struct {
	InitSuccess bool `json:"probes_initialized"`
	Features    struct {
		BpfTramp bool `json:"bpf_tramp"`
	} `json:"features"`
}

type PidInfo struct {
	Tid         int64 `json:"tid"`
	Tgid        int64 `json:"tgid"`
	Ppid        int64 `json:"ppid"`
	Pgid        int64 `json:"pgid"`
	Sid         int64 `json:"sid"`
	StartTimeNs int64 `json:"start_time_ns"`
}

type CredInfo struct {
	Ruid int64 `json:"ruid"`
	Rgid int64 `json:"rgid"`
	Euid int64 `json:"euid"`
	Egid int64 `json:"egid"`
	Suid int64 `json:"suid"`
	Sgid int64 `json:"sgid"`
}

type TtyInfo struct {
	Major int64 `json:"major"`
	Minor int64 `json:"minor"`
}

type ProcessForkEvent struct {
	ParentPids PidInfo `json:"parent_pids"`
	ChildPids  PidInfo `json:"child_pids"`
}

type ProcessExecEvent struct {
	Pids     PidInfo  `json:"pids"`
	Creds    CredInfo `json:"creds"`
	Ctty     TtyInfo  `json:"ctty"`
	FileName string   `json:"filename"`
	Cwd      string   `json:"cwd"`
	Argv     string   `json:"argv"`
}

type FileCreateEvent struct {
	Pids PidInfo `json:"pids"`
	Path string  `json:"path"`
}

type FileDeleteEvent struct {
	Pids PidInfo `json:"pids"`
	Path string  `json:"path"`
}

type FileRenameEvent struct {
	Pids    PidInfo `json:"pids"`
	OldPath string  `json:"old_path"`
	NewPath string  `json:"new_path"`
}

type SetUidEvent struct {
	Pids    PidInfo `json:"pids"`
	NewRuid int64   `json:"new_ruid"`
	NewEuid int64   `json:"new_euid"`
}

type SetGidEvent struct {
	Pids    PidInfo `json:"pids"`
	NewRgid int64   `json:"new_rgid"`
	NewEgid int64   `json:"new_egid"`
}

type TtyWriteEvent struct {
	Pids      PidInfo `json:"pids"`
	Len       int64   `json:"tty_out_len"`
	Truncated int64   `json:"tty_out_truncated"`
	Out       string  `json:"tty_out"`
}

func getJsonEventType(jsonLine string) (string, error) {
	var jsonUnmarshaled struct {
		EventType string `json:"event_type"`
	}

	err := json.Unmarshal([]byte(jsonLine), &jsonUnmarshaled)
	if err != nil {
		return "", err
	}

	return jsonUnmarshaled.EventType, nil
}

func runTestBin(binName string) []byte {
	cmd := exec.Command(fmt.Sprintf("/%s", binName))

	output, err := cmd.Output()
	if err != nil {
		fmt.Println(string(err.(*exec.ExitError).Stderr))
		fmt.Println(string(output))
		TestFail(fmt.Sprintf("Could not run test binary: %s", err))
	}

	return output
}

func AssertPidInfoEqual(tpi TestPidInfo, pi PidInfo) {
	AssertInt64Equal(pi.Tid, tpi.Tid)
	AssertInt64Equal(pi.Tgid, tpi.Tgid)
	AssertInt64Equal(pi.Ppid, tpi.Ppid)
	AssertInt64Equal(pi.Pgid, tpi.Pgid)
	AssertInt64Equal(pi.Sid, tpi.Sid)
}

func AssertTrue(val bool) {
	if val != true {
		TestFail(fmt.Sprintf("Expected %t to be true", val))
	}
}

func AssertFalse(val bool) {
	if val != false {
		TestFail(fmt.Sprintf("Expected %t to be false", val))
	}
}

func AssertStringsEqual(a, b string) {
	if a != b {
		TestFail(fmt.Sprintf("Test assertion failed %s != %s", a, b))
	}
}

func AssertInt64Equal(a, b int64) {
	if a != b {
		TestFail(fmt.Sprintf("Test assertion failed %d != %d", a, b))
	}
}

func AssertInt64NotEqual(a, b int64) {
	if a == b {
		TestFail(fmt.Sprintf("Test assertion failed %d == %d", a, b))
	}
}

func PrintBPFDebugOutput() {
	file, err := os.Open("/sys/kernel/debug/tracing/trace")
	if err != nil {
		fmt.Println("Could not open /sys/kernel/debug/tracing/trace: %s", err)
		return
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Could not read /sys/kernel/debug/tracing/trace: %s", err)
		return
	}

	fmt.Print(string(b))
}

func TestFail(v ...interface{}) {
	fmt.Println(v...)

	fmt.Println("===== STACKTRACE FOR FAILED TEST =====")
	// Don't use debug.PrintStack here. It prints to stderr, which can cause
	// Bluebox's init process to Log the stderr/stdout lines out of order (this
	// is hard on the eyes when reading). Instead manually print the stacktrace
	// to stdout so everything is going to the same stream and is serialized
	// nicely.
	b := make([]byte, 16384)
	n := runtime.Stack(b, false)
	s := string(b[:n])
	fmt.Print(s)
	fmt.Println("===== END STACKTRACE FOR FAILED TEST =====")

	fmt.Println("===== CONTENTS OF /sys/kernel/debug/tracing/trace =====")
	PrintBPFDebugOutput()
	fmt.Println("===== END CONTENTS OF /sys/kernel/debug/tracing/trace =====")

	fmt.Print("\n")
	fmt.Println("#######################################################################")
	fmt.Println("# NOTE: /sys/kernel/debug/tracing/trace will only be populated if     #")
	fmt.Println("# -DBPF_ENABLE_PRINTK was set to true in the CMake build.             #")
	fmt.Println("# CI builds do NOT enable -DBPF_ENABLE_PRINTK for performance reasons #")
	fmt.Println("#######################################################################")
	fmt.Print("\n")

	fmt.Println("BPF test failed, see errors and stacktrace above")
	os.Exit(1)
}

func AllTestsPassed() {
	fmt.Println("ALL BPF TESTS PASSED")
}

func IsOverlayFsSupported() bool {
	file, err := os.Open("/proc/filesystems")
	if err != nil {
		TestFail(fmt.Sprintf("Could not open /proc/filesystems: %s", err))
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, "overlay") {
			return true
		}
	}

	if err := scanner.Err(); err != nil {
		TestFail(fmt.Sprintf("Could not read from /proc/filesystems: %s", err))
	}

	return false
}

func RunTest(f func(*EventsTraceInstance), args ...string) {
	testFuncName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)

	et := NewEventsTrace(ctx, args...)
	et.Start(ctx)

	f(et) // Will dump info and shutdown if test fails

	// Shuts down eventstrace and goroutines listening on stdout/stderr
	cancel()

	fmt.Println("test passed: ", testFuncName)

	if err := et.Stop(); err != nil {
		TestFail(fmt.Sprintf("Could not stop EventsTrace binary: %s", err))
	}
}
