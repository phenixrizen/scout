package scout

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Scout struct {
	Services  map[uuid.UUID]*Service
	Responses chan interface{}
	Running   bool
	Logger    logrus.FieldLogger
}

type ServiceSuccess struct {
	Service   uuid.UUID
	Latency   float64
	PingTime  float64
	CreatedAt time.Time
}

type ServiceFailure struct {
	Service   uuid.UUID
	Issue     string
	PingTime  float64
	CreatedAt time.Time
	ErrorCode int
}

// NewScout returns a scout
func NewScout(servs []*Service, log logrus.FieldLogger) *Scout {
	if log == nil {
		return nil
	}
	servMap := make(map[uuid.UUID]*Service)
	resp := make(chan interface{})
	for i, serv := range servs {
		serv.Responses = resp
		if serv.Logger == nil {
			serv.Logger = log
		}
		serv.Initialize()
		servMap[serv.Id] = servs[i]
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
	if serv != nil && serv.Id != uuid.Nil {
		serv.Responses = s.Responses
		serv.Logger = logrus.New()
		s.Services[serv.Id] = serv
		if s.Running {
			go serv.Scout()
		}
	}
}

// DelService adds a service to monitor
func (s *Scout) DelService(id uuid.UUID) {
	if id != uuid.Nil {
		s.Services[id].Stop()
		delete(s.Services, id)
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
	if s, ok := s.Services[id]; ok {
		return s
	}
	return nil
}

// GetServices returns all services
func (s *Scout) GetServices() []*Service {
	servs := make([]*Service, len(s.Services))
	i := 0
	for _, serv := range s.Services {
		servs[i] = serv
		i++
	}
	return servs
}
