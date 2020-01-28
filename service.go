package scout

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	fastping "github.com/tatsushid/go-fastping"

	traceroute "github.com/phenixrizen/go-traceroute"
)

// Duration is a custom type to use for human readable durations in JSON/YAML
type Duration time.Duration

// Duration return a time.Duration
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// MarshalJSON marshals human redable durations
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON unmarshals human redable durations
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

// Service is the main struct for Services
type Service struct {
	ID               uuid.UUID              `json:"id"`
	Name             string                 `json:"name"`
	Address          string                 `json:"address"`
	Expected         string                 `json:"expected"`
	ExpectedStatus   int                    `json:"expectedStatus"`
	Interval         Duration               `json:"checkInterval"`
	Type             string                 `json:"type"`
	Method           string                 `json:"method"`
	PostData         string                 `json:"postData"`
	Port             int                    `json:"port"`
	Timeout          Duration               `json:"timeout"`
	VerifySSL        bool                   `json:"verifySSL"`
	Headers          []string               `json:"headers"`
	CreatedAt        time.Time              `json:"createdAt"`
	UpdatedAt        time.Time              `json:"updatedAt"`
	Online           bool                   `json:"online"`
	Latency          float64                `json:"latency"`
	PingTime         float64                `json:"pingTime"`
	Trace            bool                   `json:"trace"`
	TraceData        []traceroute.TraceData `json:"traceData,omitempty"`
	Retry            bool                   `json:"retry"`
	RetryMinInterval Duration               `json:"retryMinInterval"`
	RetryMaxInterval Duration               `json:"retryMaxInterval"`
	RetryMax         int                    `json:"retryMax"`
	RetryAttempts    int                    `json:"-"`
	Running          chan bool              `json:"-"`
	Checkpoint       time.Time              `json:"-"`
	SleepDuration    Duration               `json:"-"`
	LastResponse     string                 `json:"lastResponse"`
	DownText         string                 `json:"downText"`
	LastStatusCode   int                    `json:"statusCode"`
	LastOnline       time.Time              `json:"lastSuccess"`
	Logger           logrus.FieldLogger     `json:"-"`
	Responses        chan interface{}       `json:"-"`
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
	s.SleepDuration = s.Interval
ScoutLoop:
	for {
		select {
		case <-s.Running:
			s.Logger.Debugf(fmt.Sprintf("Stopping service: %v", s.Name))
			break ScoutLoop
		case <-time.After(s.SleepDuration.Duration()):
			s.Logger.Debugf("Checking: %s -> %s", s.Name, s.Type)
			s.Check()
			s.Checkpoint = s.Checkpoint.Add(s.Interval.Duration())
			sleep := Duration(s.Checkpoint.Sub(time.Now().UTC()))
			if s.Online {
				s.SleepDuration = s.Interval
			} else {
				if s.Retry {
					s.LinearJitterBackoff()
				} else {
					s.SleepDuration = sleep
				}
			}
		}
		continue
	}
}

func (s *Service) parseHost() string {
	if s.Type == "tcp" || s.Type == "udp" || s.Type == "icmp" {
		return s.Address
	} else {
		u, err := url.Parse(s.Address)
		if err != nil {
			return s.Address
		}
		return u.Hostname()
	}
}

func (s *Service) ips() []net.IP {
	var addrs []string
	var ips []net.IP
	var err error
	if s.Type == "tcp" {
		addrs, err = net.LookupHost(s.parseHost())
		if err != nil {
			return nil
		}
		for _, add := range addrs {
			ip := net.ParseIP(add)
			if ip != nil {
				ips = append(ips, ip)
			}

		}
		return ips
	} else {
		ips, err = net.LookupIP(s.parseHost())
		if err != nil {
			return nil
		}
		return ips
	}
	return nil
}

// DNSCheck will check the domain name and return a float64 representing the seconds it took to resolve DNS
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
	p.MaxRTT = s.Timeout.Duration()
	resolveIP := "ip4:icmp"
	if isIPv6(s.Address) {
		resolveIP = "ip6:icmp"
	}
	ra, err := net.ResolveIPAddr(resolveIP, s.Address)
	if err != nil {
		s.Logger.Debugf("Could not send ICMP to service %v, %v", s.Address, err)
		s.Failure(fmt.Sprintf("Could not send ICMP to service %v, %v", s.Address, err))
		return
	}
	p.AddIPAddr(ra)
	p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		s.Latency = rtt.Seconds()
		s.PingTime = rtt.Seconds()
		s.Success()
	}
	p.OnIdle = func() {
		s.Latency = -1
		s.PingTime = -1
		s.Logger.Debug("Reachmed max ICMP idle timeout")
		s.Failure("Reachmed max ICMP idle timeout")
	}
	err = p.Run()
	if err != nil {
		s.Logger.Debugf("Issue running ICMP to service %s, %v, %v", s.Name, s.Address, err)
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
	s.RetryAttempts = 0
	suc := ServiceSuccess{
		Service:   s.ID,
		Latency:   s.Latency,
		PingTime:  s.PingTime,
		CreatedAt: time.Now().UTC(),
	}
	s.Online = true
	s.Responses <- suc
}

// Failure will create a new 'ServiceFailure' record on the Response Channel
func (s *Service) Failure(issue string) {
	exhausted := false
	if s.RetryAttempts == s.RetryMax && s.RetryMax != 0 {
		s.Stop()
		exhausted = true
	}
	fail := ServiceFailure{
		Service:          s.ID,
		Issue:            issue,
		PingTime:         s.PingTime,
		RetriesExhausted: exhausted,
		CreatedAt:        time.Now().UTC(),
		ErrorCode:        s.LastStatusCode,
	}
	if s.Trace {
		ips := s.ips()
		for _, ip := range ips {
			trace := traceroute.Exec(ip, s.Timeout.Duration(), 3, 64, "icmp", 33434)
			s.TraceData = append(s.TraceData, trace)
		}
	}
	s.Online = false
	s.DownText = issue
	fail.TraceData = s.TraceData
	s.Responses <- fail
}

// LinearJitterBackoff will perform linear backoff based on the attempt number
// and with jitter to prevent a thundering herd. Min and max here are NOT
// absolute values. The number to be multipled by the attempt number will
// be chosen at random from between them, thus they are bounding the jitter.
func (s *Service) LinearJitterBackoff() {
	// RetryAttempts always starts at zero but we want to start at 1 for multiplication
	s.RetryAttempts++

	if s.RetryMaxInterval <= s.RetryMinInterval {
		// TODO think more about this...
		// if they are the same, so return min * attemptNum
		s.SleepDuration = Duration(s.RetryMinInterval.Duration() * time.Duration(s.RetryAttempts))
	}

	// Seed rand
	rand := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

	// Pick a random number that lies somewhere between the min and max and
	// multiply by the attemptNum. attemptNum starts at zero so we always
	// increment here. We first get a random percentage, then apply that to the
	// difference between min and max, and add to min.
	jitter := rand.Float64() * float64(s.RetryMaxInterval-s.RetryMinInterval)
	jitterMin := int64(jitter) + int64(s.RetryMinInterval)
	s.SleepDuration = Duration(time.Duration(jitterMin * int64(s.RetryAttempts)))
}
