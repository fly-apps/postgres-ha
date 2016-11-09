// Copyright 2015 Sorint.lab
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/sorintlab/stolon/common"
	"github.com/sorintlab/stolon/pkg/cluster"
	"github.com/sorintlab/stolon/pkg/flagutil"
	"github.com/sorintlab/stolon/pkg/store"

	"github.com/coreos/pkg/capnslog"
	"github.com/davecgh/go-spew/spew"
	"github.com/sorintlab/pollon"
	"github.com/spf13/cobra"
)

var log = capnslog.NewPackageLogger("github.com/sorintlab/stolon/cmd", "proxy")

func init() {
	capnslog.SetFormatter(capnslog.NewPrettyFormatter(os.Stderr, true))
	capnslog.SetGlobalLogLevel(capnslog.DEBUG)
}

var cmdProxy = &cobra.Command{
	Use: "stolon-proxy",
	Run: proxy,
}

type config struct {
	storeBackend   string
	storeEndpoints string
	clusterName    string
	listenAddress  string
	port           string
	stopListening  bool
	debug          bool
}

var cfg config

func init() {
	cmdProxy.PersistentFlags().StringVar(&cfg.storeBackend, "store-backend", "", "store backend type (etcd or consul)")
	cmdProxy.PersistentFlags().StringVar(&cfg.storeEndpoints, "store-endpoints", "", "a comma-delimited list of store endpoints (defaults: 127.0.0.1:2379 for etcd, 127.0.0.1:8500 for consul)")
	cmdProxy.PersistentFlags().StringVar(&cfg.clusterName, "cluster-name", "", "cluster name")
	cmdProxy.PersistentFlags().StringVar(&cfg.listenAddress, "listen-address", "127.0.0.1", "proxy listening address")
	cmdProxy.PersistentFlags().StringVar(&cfg.port, "port", "5432", "proxy listening port")
	cmdProxy.PersistentFlags().BoolVar(&cfg.stopListening, "stop-listening", true, "stop listening on store error")
	cmdProxy.PersistentFlags().BoolVar(&cfg.debug, "debug", false, "enable debug logging")
}

type ClusterChecker struct {
	id            string
	listenAddress string
	port          string

	stopListening bool

	listener         *net.TCPListener
	pp               *pollon.Proxy
	e                *store.StoreManager
	endPollonProxyCh chan error
}

func NewClusterChecker(id string, cfg config) (*ClusterChecker, error) {
	storePath := filepath.Join(common.StoreBasePath, cfg.clusterName)

	kvstore, err := store.NewStore(store.Backend(cfg.storeBackend), cfg.storeEndpoints)
	if err != nil {
		return nil, fmt.Errorf("cannot create store: %v", err)
	}
	e := store.NewStoreManager(kvstore, storePath)

	return &ClusterChecker{
		id:               id,
		listenAddress:    cfg.listenAddress,
		port:             cfg.port,
		stopListening:    cfg.stopListening,
		e:                e,
		endPollonProxyCh: make(chan error),
	}, nil
}

func (c *ClusterChecker) startPollonProxy() error {
	if c.pp != nil {
		return nil
	}

	log.Infof("Starting proxying")
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(cfg.listenAddress, cfg.port))
	if err != nil {
		return fmt.Errorf("error resolving tcp addr %q: %v", addr.String(), err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return fmt.Errorf("error listening on tcp addr %q: %v", addr.String(), err)
	}

	pp, err := pollon.NewProxy(listener)
	if err != nil {
		return fmt.Errorf("error creating pollon proxy: %v", err)
	}
	c.pp = pp
	c.listener = listener

	go func() {
		c.endPollonProxyCh <- c.pp.Start()
	}()

	return nil
}

func (c *ClusterChecker) stopPollonProxy() {
	if c.pp != nil {
		log.Infof("Stopping listening")
		c.pp.Stop()
		c.pp = nil
		c.listener.Close()
		c.listener = nil
	}
}

func (c *ClusterChecker) sendPollonConfData(confData pollon.ConfData) {
	if c.pp != nil {
		c.pp.C <- confData
	}
}

func (c *ClusterChecker) SetProxyInfo(e *store.StoreManager, uid string, generation int64, ttl time.Duration) error {
	proxyInfo := &cluster.ProxyInfo{
		UID:             c.id,
		ListenAddress:   c.listenAddress,
		Port:            c.port,
		ProxyUID:        uid,
		ProxyGeneration: generation,
	}
	log.Debugf(spew.Sprintf("proxyInfo: %#v", proxyInfo))

	if err := c.e.SetProxyInfo(proxyInfo, ttl); err != nil {
		return err
	}
	return nil
}

func (c *ClusterChecker) Check() error {
	cd, _, err := c.e.GetClusterData()
	if err != nil {
		log.Errorf("cannot get cluster data: %v", err)
		c.sendPollonConfData(pollon.ConfData{DestAddr: nil})
		if c.stopListening {
			c.stopPollonProxy()
		}
		return nil
	}
	log.Debugf(spew.Sprintf("cluster data: %#v", cd))
	if cd == nil {
		log.Infof("no clusterdata available, closing connections to previous master")
		c.sendPollonConfData(pollon.ConfData{DestAddr: nil})
		return nil
	}
	if cd.FormatVersion != cluster.CurrentCDFormatVersion {
		log.Errorf("unsupported clusterdata format version %d", cd.FormatVersion)
		c.sendPollonConfData(pollon.ConfData{DestAddr: nil})
		return nil
	}

	// Start pollon if not active
	if err = c.startPollonProxy(); err != nil {
		log.Errorf("failed to start proxy: %v", err)
		return nil
	}

	proxy := cd.Proxy
	if proxy == nil {
		log.Infof("no proxy object available, closing connections to previous master")
		c.sendPollonConfData(pollon.ConfData{DestAddr: nil})
		if err = c.SetProxyInfo(c.e, proxy.UID, proxy.Generation, 2*cluster.DefaultProxyCheckInterval); err != nil {
			log.Errorf("failed to update proxyInfo: %v", err)
		}
		return nil
	}

	db, ok := cd.DBs[proxy.Spec.MasterDBUID]
	if !ok {
		log.Infof("no db object with uid %q available, closing connections to previous master", proxy.Spec.MasterDBUID)
		c.sendPollonConfData(pollon.ConfData{DestAddr: nil})
		if err = c.SetProxyInfo(c.e, proxy.UID, proxy.Generation, 2*cluster.DefaultProxyCheckInterval); err != nil {
			log.Errorf("failed to update proxyInfo: %v", err)
		}
		return nil
	}

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%s", db.Status.ListenAddress, db.Status.Port))
	if err != nil {
		log.Errorf("err: %v", err)
		c.sendPollonConfData(pollon.ConfData{DestAddr: nil})
		return nil
	}
	log.Infof("master address: %v", addr)
	if err = c.SetProxyInfo(c.e, proxy.UID, proxy.Generation, 2*cluster.DefaultProxyCheckInterval); err != nil {
		log.Errorf("failed to update proxyInfo: %v", err)
	}

	c.sendPollonConfData(pollon.ConfData{DestAddr: addr})
	return nil
}

func (c *ClusterChecker) Start() error {
	endPollonProxyCh := make(chan error)
	checkCh := make(chan error)
	timerCh := time.NewTimer(0).C

	for true {
		select {
		case <-timerCh:
			go func() {
				checkCh <- c.Check()
			}()
		case err := <-checkCh:
			if err != nil {
				log.Debugf("check reported error: %v", err)
			}
			if err != nil {
				return fmt.Errorf("checker fatal error: %v", err)
			}
			timerCh = time.NewTimer(cluster.DefaultProxyCheckInterval).C
		case err := <-endPollonProxyCh:
			if err != nil {
				return fmt.Errorf("proxy error: %v", err)
			}
		}
	}
	return nil
}

func main() {
	flagutil.SetFlagsFromEnv(cmdProxy.PersistentFlags(), "STPROXY")

	cmdProxy.Execute()
}

func proxy(cmd *cobra.Command, args []string) {
	capnslog.SetGlobalLogLevel(capnslog.INFO)
	if cfg.debug {
		capnslog.SetGlobalLogLevel(capnslog.DEBUG)
	}
	if cfg.clusterName == "" {
		log.Fatalf("cluster name required")
	}
	if cfg.storeBackend == "" {
		log.Fatalf("store backend type required")
	}

	id := common.UID()
	log.Infof("id: %s", id)

	clusterChecker, err := NewClusterChecker(id, cfg)
	if err != nil {
		log.Fatalf("cannot create cluster checker: %v", err)
	}
	clusterChecker.Start()
}
