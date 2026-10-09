package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"esurfingdialer/internal/config"
	"esurfingdialer/internal/logger"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/session"
	"esurfingdialer/internal/state"
	"esurfingdialer/internal/utils"
)

const initialAlgoID = "00000000-0000-0000-0000-000000000000"

func Run(cfg config.Config) error {
	st := state.New()
	if err := initState(st, cfg.UserAgent); err != nil {
		return err
	}
	cli := netclient.New(st, cfg.BindInterface, cfg.BindSourceIP, cfg.BindFWMark)
	cli.SetPortalURL(cfg.PortalURL)
	sess := session.New(st, cli)
	logger.Configure(cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register before starting any worker so an early SIGTERM cannot use the
	// default process handler or leave the shutdown hook without the signal.
	signals := setupSignalHandler()
	defer signal.Stop(signals)

	dialer := NewDialer(st, cli, sess, cfg)
	dialer.Start(ctx)
	dialer.StartShutdownHook(ctx, signals, cancel)

	<-st.Done()
	cancel()
	dialer.Wait()

	return nil
}

func initState(st *state.State, userAgent string) error {
	clientID, err := utils.RandomUUIDLower()
	if err != nil {
		return err
	}
	mac, err := utils.RandomMAC()
	if err != nil {
		return err
	}
	hostName, err := utils.RandomString(10)
	if err != nil {
		return err
	}

	st.SetClientID(clientID)
	st.SetAlgoID(initialAlgoID)
	st.SetMac(mac)
	st.SetHostName(hostName)
	st.SetUserAgent(userAgent)
	return nil
}

func setupSignalHandler() chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	return ch
}
