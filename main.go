/*
* Copyright (C) 2020 The poly network Authors
* This file is part of The poly network library.
*
* The poly network is free software: you can redistribute it and/or modify
* it under the terms of the GNU Lesser General Public License as published by
* the Free Software Foundation, either version 3 of the License, or
* (at your option) any later version.
*
* The poly network is distributed in the hope that it will be useful,
* but WITHOUT ANY WARRANTY; without even the implied warranty of
* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
* GNU Lesser General Public License for more details.
* You should have received a copy of the GNU Lesser General Public License
* along with The poly network . If not, see <http://www.gnu.org/licenses/>.
 */
package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/polynetwork/kai-relayer/cmd"
	"github.com/polynetwork/kai-relayer/config"
	"github.com/polynetwork/kai-relayer/db"
	"github.com/polynetwork/kai-relayer/kaiclient"
	"github.com/polynetwork/kai-relayer/log"
	"github.com/polynetwork/kai-relayer/manager"
	sdk "github.com/polynetwork/poly-go-sdk"
	"github.com/urfave/cli"
)

var ConfigPath string
var LogDir string
var StartHeight uint64
var PolyStartHeight uint64
var StartForceHeight uint64

func setupApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "Kai relayer Service"
	app.Action = startServer
	app.Version = config.Version
	app.Copyright = "Copyright in 2019 The Ontology Authors"
	app.Flags = []cli.Flag{
		cmd.LogLevelFlag,
		cmd.ConfigPathFlag,
		cmd.KaiStartFlag,
		cmd.KaiStartForceFlag,
		cmd.PolyStartFlag,
		cmd.LogDir,
	}
	app.Commands = []cli.Command{}
	app.Before = func(context *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		return nil
	}
	return app
}

func startServer(ctx *cli.Context) {
	// get all cmd flag
	logLevel := ctx.GlobalInt(cmd.GetFlagName(cmd.LogLevelFlag))

	ld := ctx.GlobalString(cmd.GetFlagName(cmd.LogDir))
	if ld == "" {
		log.InitLog(logLevel, "./Log/", log.Stdout)
	} else {
		log.InitLog(logLevel, ld, log.Stdout)
	}

	configPath := ctx.GlobalString(cmd.GetFlagName(cmd.ConfigPathFlag))
	if configPath != "" {
		ConfigPath = configPath
	}
	kaistart := ctx.GlobalUint64(cmd.GetFlagName(cmd.KaiStartFlag))
	if kaistart > 0 {
		StartHeight = kaistart
	}

	StartForceHeight = 0
	kaistartforce := ctx.GlobalUint64(cmd.GetFlagName(cmd.KaiStartForceFlag))
	if kaistartforce > 0 {
		StartForceHeight = kaistartforce
	}
	polyStart := ctx.GlobalUint64(cmd.GetFlagName(cmd.PolyStartFlag))
	if polyStart > 0 {
		PolyStartHeight = polyStart
	}

	// read config
	servConfig := config.NewServiceConfig(ConfigPath)
	if servConfig == nil {
		log.Errorf("startServer - create config failed!")
		return
	}

	// create poly sdk
	polySdk := sdk.NewPolySdk()
	err := setUpPoly(polySdk, servConfig.PolyConfig.RestURL)
	if err != nil {
		log.Errorf("startServer - failed to setup poly sdk: %v", err)
		return
	}

	// create kai sdk
	kclient, err := kaiclient.Dial(servConfig.KAIConfig.RestURL)
	if err != nil {
		log.Errorf("startServer - cannot dial sync node, err: %s", err)
		return
	}

	var boltDB *db.BoltDB
	if servConfig.BoltDbPath == "" {
		boltDB, err = db.NewBoltDB("boltdb")
	} else {
		boltDB, err = db.NewBoltDB(servConfig.BoltDbPath)
	}
	if err != nil {
		log.Fatalf("db.NewWaitingDB error:%s", err)
		return
	}

	initKaiServer(servConfig, polySdk, kclient, boltDB)
	initPolyServer(servConfig, polySdk, kclient, boltDB)
	waitToExit()
}

func setUpPoly(poly *sdk.PolySdk, RpcAddr string) error {
	poly.NewRpcClient().SetAddress(RpcAddr)
	hdr, err := poly.GetHeaderByHeight(0)
	if err != nil {
		return err
	}
	poly.SetChainId(hdr.ChainID)
	return nil
}

func waitToExit() {
	exit := make(chan bool, 0)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for sig := range sc {
			log.Infof("waitToExit - KAI relayer received exit signal:%v.", sig.String())
			close(exit)
			break
		}
	}()
	<-exit
}

func initKaiServer(servConfig *config.ServiceConfig, polysdk *sdk.PolySdk, kclient *kaiclient.Client, boltDB *db.BoltDB) {
	mgr, err := manager.NewKardiaManager(servConfig, StartHeight, StartForceHeight, polysdk, kclient, boltDB)
	if err != nil {
		log.Error("initKAIServer - KAI service start err: %s", err.Error())
		return
	}
	go mgr.MonitorChain()
	go mgr.MonitorDeposit()
	go mgr.CheckDeposit()
}

func initPolyServer(servConfig *config.ServiceConfig, polysdk *sdk.PolySdk, kclient *kaiclient.Client, boltDB *db.BoltDB) {
	mgr, err := manager.NewPolyManager(servConfig, uint32(PolyStartHeight), polysdk, kclient, boltDB)
	if err != nil {
		log.Error("initPolyServer - PolyServer service start failed")
		return
	}
	go mgr.MonitorChain()
}

func main() {
	log.Infof("main - KAI relayer starting...")
	if err := setupApp().Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
