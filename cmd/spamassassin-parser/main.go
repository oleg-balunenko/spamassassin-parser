// spamassassin-parser is a service that shows how processing of reports works.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/obalunenko/spamassassin-parser/internal/appconfig"
	"github.com/obalunenko/spamassassin-parser/internal/fileutil"
	"github.com/obalunenko/spamassassin-parser/internal/processor"
	"github.com/obalunenko/spamassassin-parser/pkg/utils"
)

func main() {
	defer log.Println("Exit...")

	printVersion()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appCfg := appconfig.Load()

	pcCfg := processor.NewConfig()
	pcCfg.Receive.Errors = appCfg.ReceiveErrors

	pr := processor.New(pcCfg)

	go pr.Process(ctx)

	fileChan := make(chan string)

	go fileutil.PollDirectory(ctx, appCfg.InputDir, availableExtensions, fileChan)

	go func(ctx context.Context, fileChan chan string) {
		for {
			select {
			case <-ctx.Done():
				return

			case reportFile := <-fileChan:
				file, err := os.Open(filepath.Clean(filepath.Join(appCfg.InputDir, reportFile)))
				if err != nil {
					log.Fatal(fmt.Errorf("failed to open file with report: %w", err))
				}

				go func() {
					pr.Input() <- processor.NewInput(file, filepath.Base(file.Name()))
				}()
			}
		}
	}(ctx, fileChan)

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var wg sync.WaitGroup

	waitRoutinesNum := 1

	wg.Add(waitRoutinesNum)

	go process(ctx, &wg, pr, appCfg)

	s := <-stopChan
	log.Infof("Signal [%s] received", s.String())

	cancel()

	wg.Wait()
}

func process(ctx context.Context, wg *sync.WaitGroup, pr processor.Processor, dirsCfg appconfig.Config) {
	defer wg.Done()

	for {
		select {
		case res := <-pr.Results():
			if res != nil {
				s, err := utils.PrettyPrint(res.Report, "", "\t")
				if err != nil {
					log.Error(fmt.Errorf("failed to print report: %w", err))
				}

				log.Printf("[TestID: %s] archive: \n %s \n",
					res.TestID, s)

				if err = fileutil.WriteFile(res.TestID, dirsCfg.ResultDir, s); err != nil {
					log.Error(fmt.Errorf("failed to write file: %w", err))
				}

				log.Infof("Moving file %s to archive folder: %s", res.TestID, dirsCfg.ArchiveDir)

				if err = fileutil.MoveFile(res.TestID, dirsCfg.InputDir, dirsCfg.ArchiveDir); err != nil {
					log.Error(fmt.Errorf("failed to move archive file: %w", err))
				}

				log.Info("File moved")
			}

		case err := <-pr.Errors():
			if err != nil {
				log.Error(err)
			}

		case <-ctx.Done():
			log.Println("context canceled")

			pr.Close()

			return
		}
	}
}

var availableExtensions = map[string]bool{
	"txt": true,
}
