package main

import (
	"io/ioutil"
	"time"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"

	"github.com/phenixrizen/scout"
)

func main() {
	log := logrus.New()

	var servs []*scout.Service
	yb, err := ioutil.ReadFile("./services.yml")
	if err != nil {
		logrus.Fatal(err)
	}
	err = yaml.Unmarshal(yb, &servs)
	if err != nil {
		logrus.Fatal(err)
	}

	s := scout.NewScout(servs, log)

	go s.StartScoutingServices()
	go s.HandleResponses()

	for {
		time.Sleep(30 * time.Second)
		for _, serv := range s.Services {
			log.Infof("Service: %s, Address: %s, Type: %s, Online: %t, Last Online: %s, Last Status Code: %d, Latency: %.6fs, Ping Time: %.6fs", serv.Name, serv.Address, serv.Type, serv.Online, serv.LastOnline, serv.LastStatusCode, serv.Latency, serv.PingTime)
		}
	}
}
