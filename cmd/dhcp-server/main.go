package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-sense/go-sense/pkg/log"

	"github.com/coreos/etcd/clientv3"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func main() {
	level := zap.LevelFlag("log-level", zap.ErrorLevel, fmt.Sprintf("Set loglevel. Possible values: %s", strings.Join([]string{zap.DebugLevel.String(), zap.InfoLevel.String(), zap.WarnLevel.String(), zap.ErrorLevel.String(), zap.PanicLevel.String(), zap.FatalLevel.String()}, ", ")))
	flag.Parse()

	logger, sync, err := log.GetLogger(*level)
	if err != nil {
		fmt.Printf("failed to create logger: %v", err)
		os.Exit(1)
	}
	defer sync()

	ctx, cancel := context.WithCancel(context.Background())
	g := run.Group{}
	g.Add(func() error {
		logger.Info("DHCP Server!")

		client, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"localhost:2379"},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return errors.Wrap(err, "failed to create etcd client for config")
		}
		defer client.Close()

		if _, err = client.Put(context.Background(), "foo", "bar"); err != nil {
			return errors.Wrap(err, "could not create key 'foo'")
		}
		logger.Info("wrote key foo")

		res, err := client.Get(context.Background(), "foo")
		if err != nil {
			return errors.Wrap(err, "could not read key 'foo'")
		}

		logger.Infof("Count: %d", res.Count)
		logger.Infof("%s=%s", res.Kvs[0].Key, res.Kvs[0].Value)

		<-ctx.Done()
		return nil
	}, func(error) {
		cancel()
	})

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	g.Add(func() error {
		<-c
		logger.Info("Received interrupt")
		return nil
	}, func(error) {
		close(c)
	})

	if err := g.Run(); err != nil {
		logger.Errorf("Error during runtime: %v", err)
	}
}
