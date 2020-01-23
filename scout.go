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
	Logger    *logrus.Logger
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

func NewScout(servs []*Service, log *logrus.Logger) *Scout {
	if log == nil {
		log = logrus.New()
	}
	servMap := make(map[uuid.UUID]*Service)
	resp := make(chan interface{})
	for i, serv := range servs {
		serv.Responses = resp
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

// CheckServices will start the checking go routine for each service
func (s *Scout) CheckServices() {
	s.Logger.Infoln(fmt.Sprintf("Starting monitoring process for %v Services", len(s.Services)))
	for _, ser := range s.Services {
		go ser.Scout()
	}
}

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
