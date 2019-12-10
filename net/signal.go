package net

import (
	log "github.com/sirupsen/logrus"
	"ma-commons-go/utils"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	wg sync.WaitGroup
	// server close signal
	Die = make(chan struct{})
)

func SigDone() {
	wg.Done()
}

func SigAdd() {
	wg.Add(1)
}

// handle unix signals
func SigHandler(exit func()) {
	defer utils.PrintPanicStack()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM)

	for {
		msg := <-ch
		switch msg {
		case syscall.SIGTERM:
			exit()
			close(Die)
			log.Info("sigterm received")
			log.Info("waiting for server close, please wait...")
			wg.Wait()
			log.Info("server shutdown.")
			os.Exit(0)
		}
	}
}
