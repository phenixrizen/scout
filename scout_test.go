package scout

import (
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestScout(t *testing.T) {
	assert := assert.New(t)

	log := logrus.New()

	google := &Service{
		ID:             uuid.New(),
		Name:           "Google",
		Address:        "https://google.com",
		Timeout:        5,
		Interval:       5000,
		ExpectedStatus: 200,
		Type:           "http",
		Logger:         log,
	}

	netlify := &Service{
		ID:             uuid.New(),
		Name:           "Netlify",
		Address:        "https://netlify.com",
		Timeout:        5,
		Interval:       4000,
		ExpectedStatus: 200,
		Type:           "http",
		Logger:         log,
	}

	netlifyPing := &Service{
		ID:       uuid.New(),
		Name:     "Netlify",
		Address:  "netlify.com",
		Timeout:  5,
		Interval: 3000,
		Type:     "icmp",
		Logger:   log,
	}

	servs := []*Service{google, netlify, netlifyPing}

	s := NewScout(servs, log)
	assert.NotNil(s)

	// go s.CheckServices()
	//# s.HandleResponses()

}
