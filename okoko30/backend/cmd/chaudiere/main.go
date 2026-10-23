package main

import (
	"chaudiere/internal/app"
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := flag.String("config", "config.json", "configuration JSON")
	flag.Parse()
	c, e := app.LoadConfig(*cfg)
	if e != nil {
		log.Fatal(e)
	}
	a, e := app.New(c)
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if c.BoilerURL != "" {
		go a.RunCollector(ctx)
	}
	go a.RunCSVImporter(ctx)
	if c.BoilerURL != "" {
		go a.RunLogImporter(ctx)
	}
	if e = a.Serve(ctx); e != nil {
		log.Fatal(e)
	}
}
