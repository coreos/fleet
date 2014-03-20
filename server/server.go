package server

import (
	"encoding/json"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
)

type Server struct {
	agent       *agent.Agent
	engine      *engine.Engine
	machine     *machine.Machine
	registry    *registry.Registry
	eventBus    *event.EventBus
	eventStream *registry.EventStream
}

func New(cfg config.Config) *Server {
	m := machine.New(cfg.BootID, cfg.PublicIP, cfg.Metadata())
	m.RefreshState()

	regClient := etcd.NewClient(cfg.EtcdServers)
	regClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	r := registry.New(regClient)

	eb := event.NewEventBus()
	eb.Listen()

	eventClient := etcd.NewClient(cfg.EtcdServers)
	eventClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	es := registry.NewEventStream(eventClient, r)

	var verifier *sign.SignatureVerifier
	if cfg.VerifyUnits {
		var err error
		verifier, err = sign.NewSignatureVerifierFromAuthorizedKeysFile(cfg.AuthorizedKeysFile)
		if err != nil {
			log.Errorln("Failed to get any key from authorized key file in verify_units mode:", err)
			verifier = sign.NewSignatureVerifier()
		}
	}

	a, err := agent.New(r, eb, m, cfg.AgentTTL, cfg.UnitPrefix, verifier)
	if err != nil {
		//TODO: return this as an error object rather than panicking
		panic(err)
	}

	e := engine.New(r, eb, m)

	return &Server{a, e, m, r, eb, es}
}

func (self *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Agent *agent.Agent }{Agent: self.agent})
}

func (self *Server) Run() {
	go self.agent.Run()
	go self.engine.Run()

	go self.eventBus.Listen()
	go self.eventStream.Stream(self.eventBus.Channel)
}

func (self *Server) Stop() {
	self.agent.Stop()
	self.engine.Stop()

	self.eventStream.Close()
	self.eventBus.Stop()
}

func (self *Server) Purge() {
	self.agent.Purge()
}
