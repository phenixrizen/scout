## scout - Simple checking of http, tcp, udp, connections and icmp checks
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/phenixrizen/scout)
[![Build Status](https://travis-ci.org/phenixrizen/scout.svg?branch=master)](https://travis-ci.org/phenixrizen/scout)
[![Coverage Status](https://coveralls.io/repos/github/phenixrizen/scout/badge.svg?branch=master)](https://coveralls.io/github/phenixrizen/scout?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/phenixrizen/scout)](https://goreportcard.com/report/github.com/phenixrizen/scout)

### Key Features
- Ability to monitor multiple services
- Ability to monitor tcp, udp, http, and icmp
- Ability to add and remove services for monitoring
- Ability to specify expected response content and codes
- Ability to specify check interval and timeouts per service

### Get Started

#### Installation
```bash
$ go get github.com/phenixrizen/scout
```

#### Example Usage
```go

package main

import (
	"io/ioutil"
	"time"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"

	"github.com/phenixrizen/scout"
)
[]
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
```

#### Example Services YAML
```yaml
---
- id: 8b3c6416-2578-4418-8cbf-a8424e7ce04d
  name: Google
  address: https://google.com
  expected: ''
  expectedStatus: 200
  checkInterval: 5000
  type: http
  timeout: 5
- id: 409455e9-c496-4907-8478-34cff2e7b131
  name: Netlify
  address: https://netlify.com
  expected: ''
  expectedStatus: 200
  checkInterval: 4000
  type: http
- id: fe727692-bde3-4021-819b-1ceedad4aa27
  name: Netlify
  address: netlify.com
  checkInterval: 3000
  type: icmp
```