package scout

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	traceroute "github.com/phenixrizen/go-traceroute"
)

type Scout struct {
	Services  map[uuid.UUID]*Service
	Responses chan interface{}
	Running   bool
	Logger    logrus.FieldLogger
	mux       sync.RWMutex
}

type ServiceSuccess struct {
	Service   uuid.UUID `json:"service"`
	Latency   float64   `json:"latency"`
	PingTime  float64   `json:"pingTime"`
	CreatedAt time.Time `json:"createdAt"`
}

type ServiceFailure struct {
	Service          uuid.UUID              `json:"service"`
	Issue            string                 `json:"issue"`
	PingTime         float64                `json:"pingTime"`
	TraceData        []traceroute.TraceData `json:"traceData,omitempty"`
	RetriesExhausted bool                   `json:"retiresExhausted,omitempty`
	CreatedAt        time.Time              `json:"createdAt"`
	ErrorCode        int                    `json:"errorCode,omitempty"`
}

// NewScout returns a scout
func NewScout(servs []*Service, log logrus.FieldLogger) *Scout {
	if log == nil {
		return nil
	}
	log = log.WithField("component", "scout")
	servMap := make(map[uuid.UUID]*Service)
	resp := make(chan interface{})
	for i, serv := range servs {
		serv.Responses = resp
		if serv.Logger == nil {
			serv.Logger = log
		}
		serv.Initialize()
		servMap[serv.ID] = servs[i]
	}
	s := &Scout{
		Services:  servMap,
		Responses: resp,
		Logger:    log,
	}

	return s
}

// AddService adds a service to monitor
func (s *Scout) AddService(serv *Service) {
	if serv != nil && serv.ID != uuid.Nil {
		serv.Responses = s.Responses
		serv.Logger = s.Logger
		s.mux.Lock()
		s.Services[serv.ID] = serv
		if s.Running {
			go serv.Scout()
		}
		s.mux.Unlock()
	}
}

// DelService adds a service to monitor
func (s *Scout) DelService(id uuid.UUID) {
	if id != uuid.Nil {
		s.Services[id].Stop()
		s.mux.Lock()
		delete(s.Services, id)
		s.mux.Unlock()
	}
}

// StartScoutingServices will start the checking go routine for each service
func (s *Scout) StartScoutingServices() {
	s.Logger.Infof(fmt.Sprintf("Starting scouting routines for %v Services", len(s.Services)))
	if !s.Running {
		for _, ser := range s.Services {
			go ser.Scout()
		}
		s.Running = true
	}
}

// StopScoutingServices will start the checking go routine for each service
func (s *Scout) StopScoutingServices() {
	s.Logger.Infof(fmt.Sprintf("Stopping scouting routines for %v Services", len(s.Services)))
	if s.Running {
		for _, ser := range s.Services {
			ser.Stop()
		}
		s.Running = false
	}
}

// GetResponseChannel returns a interface channel that has either ServiceSuccess or ServiceFailure responses
func (s *Scout) GetResponseChannel() chan interface{} {
	return s.Responses
}

// HandleResponses simply logs current responses, this is not intended to be used, but demonatrates scouts usage
func (s *Scout) HandleResponses() {
	s.Logger.Info("Listening for Responses...")
	for resp := range s.Responses {
		success, ok := resp.(ServiceSuccess)
		if ok {
			s.Logger.Infof("Response: SUCCESS %s -> %s %+v", s.Services[success.Service].Name, s.Services[success.Service].Type, resp)
			continue
		}
		fail, ok := resp.(ServiceFailure)
		if ok {
			s.Logger.Infof("Response: FAILURE %s -> %s %+v", s.Services[fail.Service].Name, s.Services[fail.Service].Type, resp)
			continue
		}
	}
}

// GetService returns a service
func (s *Scout) GetService(id uuid.UUID) *Service {
	s.mux.RLock()
	if s, ok := s.Services[id]; ok {
		return s
	}
	s.mux.RUnlock()
	return nil
}

// GetServices returns all services
func (s *Scout) GetServices() []*Service {
	s.mux.RLock()
	servs := make([]*Service, len(s.Services))
	i := 0
	for _, serv := range s.Services {
		servs[i] = serv
		i++
	}
	s.mux.RUnlock()
	return servs
}
