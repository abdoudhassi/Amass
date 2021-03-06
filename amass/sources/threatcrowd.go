// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package sources

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OWASP/Amass/amass/core"
	"github.com/OWASP/Amass/amass/utils"
)

// ThreatCrowd is the Service that handles access to the ThreatCrowd data source.
type ThreatCrowd struct {
	core.BaseService

	SourceType string
}

// NewThreatCrowd returns he object initialized, but not yet started.
func NewThreatCrowd(config *core.Config, bus *core.EventBus) *ThreatCrowd {
	t := &ThreatCrowd{SourceType: core.API}

	t.BaseService = *core.NewBaseService(t, "ThreatCrowd", config, bus)
	return t
}

// OnStart implements the Service interface
func (t *ThreatCrowd) OnStart() error {
	t.BaseService.OnStart()

	go t.processRequests()
	return nil
}

func (t *ThreatCrowd) processRequests() {
	for {
		select {
		case <-t.Quit():
			return
		case req := <-t.RequestChan():
			if t.Config().IsDomainInScope(req.Domain) {
				t.executeQuery(req.Domain)
			}
		}
	}
}

func (t *ThreatCrowd) executeQuery(domain string) {
	url := t.getURL(domain)
	headers := map[string]string{"Content-Type": "application/json"}
	page, err := utils.RequestWebPage(url, nil, headers, "", "")
	if err != nil {
		t.Config().Log.Printf("%s: %s: %v", t.String(), url, err)
		return
	}

	// Extract the subdomain names and IP addresses from the results
	var m struct {
		ResponseCode string   `json:"response_code"`
		Subdomains   []string `json:"subdomains"`
		Resolutions  []struct {
			IP string `json:"ip_address"`
		} `json:"resolutions"`
	}
	if err := json.Unmarshal([]byte(page), &m); err != nil {
		return
	}

	if m.ResponseCode != "1" {
		t.Config().Log.Printf("%s: %s: Response code %s", t.String(), url, m.ResponseCode)
		return
	}

	t.SetActive()
	re := t.Config().DomainRegex(domain)
	for _, sub := range m.Subdomains {
		s := strings.ToLower(sub)

		if re.MatchString(s) {
			t.Bus().Publish(core.NewNameTopic, &core.Request{
				Name:   s,
				Domain: domain,
				Tag:    t.SourceType,
				Source: t.String(),
			})
		}
	}

	for _, res := range m.Resolutions {
		t.Bus().Publish(core.NewAddrTopic, &core.Request{
			Address: res.IP,
			Domain:  domain,
			Tag:     t.SourceType,
			Source:  t.String(),
		})
	}
}

func (t *ThreatCrowd) getURL(domain string) string {
	format := "https://www.threatcrowd.org/searchApi/v2/domain/report/?domain=%s"

	return fmt.Sprintf(format, domain)
}
