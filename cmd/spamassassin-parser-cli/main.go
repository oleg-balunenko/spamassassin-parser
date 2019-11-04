package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/pkg/errors"

	"github.com/oleg-balunenko/spamassassin-parser/pkg/processor"
	"github.com/oleg-balunenko/spamassassin-parser/pkg/utils"
)

var (
	reportFile = flag.String("report_file", "", "path to report file to process")
)

func main() {
	printVersion()
	defer log.Println("Exit...")

	flag.Parse()

	if *reportFile == "" {
		log.Fatal("report_file not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	req := processor.MakeInputChan()
	resChan := processor.MakeResponseChan()

	go processor.ProcessReports(ctx, req)

	file, err := os.Open(*reportFile)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to open file with report"))
	}
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	go func() {
		req <- processor.Input{
			Data:       file,
			TestID:     file.Name(),
			ResultChan: resChan,
		}
		close(req)
	}()

LOOP:
	for {
		select {
		case res := <-resChan:
			if res.Error != nil {
				log.Fatalf("%s: %v \n", res.TestID, res.Error)
			}
			s, err := utils.PrettyPrint(res.Report, "", "\t")
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed to print report"))
			}
			log.Printf("TestID[%s]:\n %s", res.TestID, s)
		case <-ctx.Done():
			log.Println("context deadline")
			break LOOP
		case <-stopChan:
			log.Println("ctrl+c received")
			break LOOP
		}
	}

}
