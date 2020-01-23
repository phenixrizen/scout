package main

import (
	"github.com/google/uuid"
	"github.com/phenixrizen/scout"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()

	google := &scout.Service{
		Id:             uuid.New(),
		Name:           "Google",
		Address:        "https://google.com",
		Timeout:        5,
		Interval:       5000,
		ExpectedStatus: 200,
		Type:           "http",
		Logger:         log,
	}

	netlify := &scout.Service{
		Id:             uuid.New(),
		Name:           "Netlify",
		Address:        "https://netlify.com",
		Timeout:        5,
		Interval:       4000,
		ExpectedStatus: 200,
		Type:           "http",
		Logger:         log,
	}

	netlifyPing := &scout.Service{
		Id:       uuid.New(),
		Name:     "Netlify",
		Address:  "netlify.com",
		Timeout:  5,
		Interval: 3000,
		Type:     "icmp",
		Logger:   log,
	}

	servs := []*scout.Service{google, netlify, netlifyPing}

	s := scout.NewScout(servs, log)

	go s.CheckServices()
	s.HandleResponses()
}
