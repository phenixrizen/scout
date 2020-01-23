package scout

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	fastping "github.com/tatsushid/go-fastping"
)

// Service is the main struct for Services
type Service struct {
	Id             uuid.UUID        `json:"id"`
	Name           string           `json:"name"`
	Address        string           `json:"address"`
	Expected       string           `json:"expected"`
	ExpectedStatus int              `json:"expectedStatus"`
	Interval       int              `json:"checkInterval"`
	Type           string           `json:"type"`
	Method         string           `json:"method"`
	PostData       string           `json:"postData"`
	Port           int              `json:"port"`
	Timeout        int              `json:"timeout"`
	VerifySSL      bool             `json:"verifySSL"`
	Headers        []string         `json:"headers"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
	Online         bool             `json:"online"`
	Latency        float64          `json:"latency"`
	PingTime       float64          `json:"pingTime"`
	Running        chan bool        `json:"-"`
	Checkpoint     time.Time        `json:"-"`
	SleepDuration  time.Duration    `json:"-"`
	LastResponse   string           `json:"-"`
	DownText       string           `json:"-"`
	LastStatusCode int              `json:"statusCode"`
	LastOnline     time.Time        `json:"lastSuccess"`
	Logger         *logrus.Logger   `json:"-"`
	Responses      chan interface{} `json:"-"`
}

// Initialize a Service
func (s *Service) Initialize() {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
		s.UpdatedAt = time.Now().UTC()
	}
}

// Start will create a channel for use to stop the service checking go routine
func (s *Service) Start() {
	s.Running = make(chan bool)
}

// Stop will stop the go routine that is checking if service is online or not
func (s *Service) Stop() {
	if s.IsRunning() {
		close(s.Running)
	}
}

// IsRunning returns true if the service go routine is running
func (s *Service) IsRunning() bool {
	if s.Running == nil {
		return false
	}
	select {
	case <-s.Running:
		return false
	default:
		return true
	}
}

// Check will run checkHttp for HTTP services and checkTcp for TCP services
func (s *Service) Check() {
	switch s.Type {
	case "http":
		s.CheckHTTP()
	case "tcp", "udp":
		s.CheckNet()
	case "icmp":
		s.CheckICMP()
	}
}

// Scout is the main go routine for checking a service
func (s *Service) Scout() {
	s.Start()
	s.Checkpoint = time.Now().UTC()
	s.SleepDuration = (time.Duration(s.Interval) * time.Millisecond)
ScoutLoop:
	for {
		select {
		case <-s.Running:
			s.Logger.Infof(fmt.Sprintf("Stopping service: %v", s.Name))
			break ScoutLoop
		case <-time.After(s.SleepDuration):
			s.Logger.Infof("Checking: %s -> %s", s.Name, s.Type)
			s.Check()
			s.Checkpoint = s.Checkpoint.Add(s.duration())
			sleep := s.Checkpoint.Sub(time.Now().UTC())
			if !s.Online {
				s.SleepDuration = s.duration()
			} else {
				s.SleepDuration = sleep
			}
		}
		continue
	}
}

// duration returns the amount of duration for a service to check its status
func (s *Service) duration() time.Duration {
	return time.Duration(s.Interval) * time.Millisecond
}

func (s *Service) parseHost() string {
	if s.Type == "tcp" || s.Type == "udp" {
		return s.Address
	} else {
		u, err := url.Parse(s.Address)
		if err != nil {
			return s.Address
		}
		return u.Hostname()
	}
}

// DNSCheck will check the domain name and return a float64 representing the seconds it took to reslove DNS
func (s *Service) DNSCheck() (float64, error) {
	var err error
	t1 := time.Now()
	host := s.parseHost()
	if s.Type == "tcp" {
		_, err = net.LookupHost(host)
	} else {
		_, err = net.LookupIP(host)
	}
	if err != nil {
		return 0, err
	}
	t2 := time.Now()
	subTime := t2.Sub(t1).Seconds()
	return subTime, err
}

func isIPv6(address string) bool {
	return strings.Count(address, ":") >= 2
}

// CheckICMP will send a ICMP ping packet to the service
func (s *Service) CheckICMP() {
	p := fastping.NewPinger()
	resolveIP := "ip4:icmp"
	if isIPv6(s.Address) {
		resolveIP = "ip6:icmp"
	}
	ra, err := net.ResolveIPAddr(resolveIP, s.Address)
	if err != nil {
		s.Failure(fmt.Sprintf("Could not send ICMP to service %v, %v", s.Address, err))
		return
	}
	p.AddIPAddr(ra)
	p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		s.Latency = rtt.Seconds()
		s.PingTime = rtt.Seconds()
		s.Success()
	}
	err = p.Run()
	if err != nil {
		s.Failure(fmt.Sprintf("Issue running ICMP to service %v, %v", s.Address, err))
		return
	}
	s.LastResponse = ""
}

// CheckNet will check a TCP/UDP service
func (s *Service) CheckNet() {
	dnsLookup, err := s.DNSCheck()
	if err != nil {
		s.Failure(fmt.Sprintf("Could not get IP address for TCP service %v, %v", s.Address, err))
		return
	}
	s.PingTime = dnsLookup
	t1 := time.Now()
	domain := fmt.Sprintf("%v", s.Address)
	if s.Port != 0 {
		domain = fmt.Sprintf("%v:%v", s.Address, s.Port)
		if isIPv6(s.Address) {
			domain = fmt.Sprintf("[%v]:%v", s.Address, s.Port)
		}
	}
	conn, err := net.DialTimeout(s.Type, domain, time.Duration(s.Timeout)*time.Second)
	if err != nil {
		s.Failure(fmt.Sprintf("Dial Error %v", err))
		return
	}
	if err := conn.Close(); err != nil {
		s.Failure(fmt.Sprintf("%v Socket Close Error %v", strings.ToUpper(s.Type), err))
		return
	}
	t2 := time.Now()
	s.Latency = t2.Sub(t1).Seconds()
	s.LastResponse = ""
	s.Success()
}

// CheckHTTP will check a HTTP service
func (s *Service) CheckHTTP() {
	dnsLookup, err := s.DNSCheck()
	if err != nil {
		s.Failure(fmt.Sprintf("Could not get IP address for domain %v, %v", s.Address, err))
		return
	}
	s.PingTime = dnsLookup
	t1 := time.Now()

	timeout := time.Duration(s.Timeout) * time.Second
	var content []byte
	var res *http.Response

	if s.Method == "POST" {
		content, res, err = HttpRequest(s.Address, s.Method, "application/json", s.Headers, bytes.NewBuffer([]byte(s.PostData)), timeout, s.VerifySSL)
	} else {
		content, res, err = HttpRequest(s.Address, s.Method, nil, s.Headers, nil, timeout, s.VerifySSL)
	}
	if err != nil {
		s.Failure(fmt.Sprintf("HTTP Error %v", err))
		return
	}
	t2 := time.Now()
	s.Latency = t2.Sub(t1).Seconds()
	s.LastResponse = string(content)
	s.LastStatusCode = res.StatusCode

	if s.Expected != "" {
		match, err := regexp.MatchString(s.Expected, string(content))
		if err != nil {
			s.Logger.Warnln(fmt.Sprintf("Service %v expected: %v to match %v", s.Name, string(content), s.Expected))
		}
		if !match {
			s.Failure(fmt.Sprintf("HTTP Response Body did not match '%v'", s.Expected))
			return
		}
	}
	if s.ExpectedStatus != res.StatusCode {
		s.Failure(fmt.Sprintf("HTTP Status Code %v did not match %v", res.StatusCode, s.ExpectedStatus))
		return
	}

	s.Success()
}

// Success will create a new 'ServiceSuccess' record on the Response Channel
func (s *Service) Success() {
	s.LastOnline = time.Now().UTC()
	suc := ServiceSuccess{
		Service:   s.Id,
		Latency:   s.Latency,
		PingTime:  s.PingTime,
		CreatedAt: time.Now().UTC(),
	}
	s.Online = true
	s.Responses <- suc
}

// Failure will create a new 'ServiceFailure' record on the Response Channel
func (s *Service) Failure(issue string) {
	fail := ServiceFailure{
		Service:   s.Id,
		Issue:     issue,
		PingTime:  s.PingTime,
		CreatedAt: time.Now().UTC(),
		ErrorCode: s.LastStatusCode,
	}
	s.Online = false
	s.DownText = issue
	s.Responses <- fail
}
