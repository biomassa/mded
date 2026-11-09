//go:build prof

package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"syscall"
)

func init() {
	go func() { // SIGUSR1 appends the goroutine count and stacks to /tmp/mded-goroutines.txt
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGUSR1)
		for range ch {
			f, _ := os.OpenFile("/tmp/mded-goroutines.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			fmt.Fprintf(f, "=== %d goroutines\n", runtime.NumGoroutine())
			pprof.Lookup("goroutine").WriteTo(f, 1)
			f.Close()
		}
	}()
	if p := os.Getenv("MDED_GOROUTINES"); p != "" {
		defer func() {
			prev := stopProf
			stopProf = func() {
				f, _ := os.Create(p)
				pprof.Lookup("goroutine").WriteTo(f, 1)
				f.Close()
				if prev != nil {
					prev()
				}
			}
		}()
	}
	if p := os.Getenv("MDED_CPUPROF"); p != "" {
		f, _ := os.Create(p)
		pprof.StartCPUProfile(f)
		stopProf = func() { pprof.StopCPUProfile(); f.Close() }
	}
	if p := os.Getenv("MDED_TRACE"); p != "" {
		f, _ := os.Create(p)
		trace.Start(f)
		prev := stopProf
		stopProf = func() {
			trace.Stop()
			f.Close()
			if prev != nil {
				prev()
			}
		}
	}
}
