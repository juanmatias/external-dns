/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Provider for SHAMAN DNS

More info here: https://github.com/nanopack/shaman

This work is based on the one from CloudFlare, thanks to them.

Author: Juan Matias KungFu de la Camara Beovide <juanmatias@gmail.com>
Company: 3XM Group: https://www.3xmgroup.com/

March 2020
This code is COVID-19 free, but use it at your own risk.

*/
package shaman

import (
	"context"
	//"fmt"
	"os"
	"testing"
    "errors"

    shaman "gitlab.com/tooling2/shaman-client-go"
	"github.com/stretchr/testify/assert"
/*-	"github.com/stretchr/testify/require"
*/
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

/* *********************************************
 Values for tests
*************/
func values2test_domains () shaman.DomainsResponse {
    return shaman.DomainsResponse{
        "nanopack.io",
        "www.3xmgroup.com",
        "juanmatiasdelacamara.wordpress.com",
    }
}

func values2test_records () []shaman.DomainResponse {
            return []shaman.DomainResponse{
                    {
                        Domain: "nanopack.io",
                        Records: []shaman.DNSRecordResponse{
                            {
                            TTL: 60,
                            Class: "IN",
                            Type: "A",
                            Address: "P. Sherman 42 Wallaby Way",
                            },
                        },
                    },
                    {
                        Domain: "www.3xmgroup.com",
                        Records: []shaman.DNSRecordResponse{
                            {
                            TTL: 60,
                            Class: "IN",
                            Type: "A",
                            Address: "0001 Cemetery Lane",
                            },
                        },
                    },
                    {
                        Domain: "juanmatiasdelacamara.wordpress.com",
                        Records: []shaman.DNSRecordResponse{
                            {
                            TTL: 60,
                            Class: "IN",
                            Type: "A",
                            Address: "Bag End, Bagshot Row, Hobbiton",
                            },
                        },
                    },
                }
}

/* **********
 Const for tests
************************************************/

/* *********************************************
 Mock client
*************/
type mockShamanClient struct{}

func (m *mockShamanClient) GetDomains() (shaman.DomainsResponse, error) {
    domains := values2test_domains()
    return domains, nil
}

func (m *mockShamanClient) GetRecords(domainName string) (shaman.DomainResponse, error) {
    records := values2test_records()
    for _, r := range(records) {
        return r, nil
    }
    return shaman.DomainResponse{}, nil
}
func (m *mockShamanClient) DeleteDomain(domainName string) (shaman.MsgResponse, error) {
    return shaman.MsgResponse{}, nil
}
func (m *mockShamanClient) CreateDomain(name string) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, nil
}
func (m *mockShamanClient) CreateRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, nil
}
func (m *mockShamanClient) CreateDomainWithRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, nil
}
func (m *mockShamanClient) ReplaceDomains(domainNames []string) ([]shaman.DomainResponse, error) {
    return []shaman.DomainResponse{}, nil
}
func (m *mockShamanClient) ReplaceDomainsWithRecords(replacementDomains []shaman.Domain) ([]shaman.DomainResponse, error) {
    return []shaman.DomainResponse{}, nil
}
func (m *mockShamanClient) ReplaceDomainRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, nil
}

type mockShamanClientFail struct{}

func (m *mockShamanClientFail) GetDomains() (shaman.DomainsResponse, error) {
    return shaman.DomainsResponse{}, errors.New("Can't get domains")
}

func (m *mockShamanClientFail) GetRecords(domainName string) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, errors.New("Where in the World Is Carmen Sandiego")
}
func (m *mockShamanClientFail) DeleteDomain(domainName string) (shaman.MsgResponse, error) {
    return shaman.MsgResponse{}, errors.New("Be yourself, everyone else is already taken")
}
func (m *mockShamanClientFail) CreateDomain(name string) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, errors.New("So many books, so little time")
}
func (m *mockShamanClientFail) CreateRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, errors.New("You know you're in love when you can't fall asleep because reality is finally better than your dreams")
}
func (m *mockShamanClientFail) CreateDomainWithRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, errors.New("Be the change that you wish to see in the world")
}
func (m *mockShamanClientFail) ReplaceDomains(domainNames []string) ([]shaman.DomainResponse, error) {
    return []shaman.DomainResponse{}, errors.New("If you tell the truth, you don't have to remember anything")
}
func (m *mockShamanClientFail) ReplaceDomainsWithRecords(replacementDomains []shaman.Domain) ([]shaman.DomainResponse, error) {
    return []shaman.DomainResponse{}, errors.New("Without music, life would be a mistake")
}
func (m *mockShamanClientFail) ReplaceDomainRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return shaman.DomainResponse{}, errors.New("I may not have gone where I intended to go, but I think I have ended up where I needed to be")
}
/* ********
 Mock client
************************************************/

/* *********************************************
 Test functions
*************/

func TestShamanNewShamanProvider(t *testing.T) {
    baseURL := ""
	_, err := shaman.NewWithAPIToken(os.Getenv("SHAMAN_API_TOKEN"), baseURL)
	if err == nil {
		t.Errorf("should fail, %s", err)
	}
	_ = os.Setenv("SHAMAN_API_TOKEN", "secret")
	_, err = shaman.NewWithAPIToken(os.Getenv("SHAMAN_API_TOKEN"), baseURL)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	_ = os.Setenv("SHAMAN_API_TOKEN", "secret")
    baseURL = "shaman.org:1632"
	_, err = shaman.NewWithAPIToken(os.Getenv("SHAMAN_API_TOKEN"), baseURL)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}
}

func TestShamanGetDomains (t *testing.T) {
	provider := &ShamanProvider{
		Client: &mockShamanClient{},
	}
	ctx := context.Background()

	records, err := provider.Domains(ctx)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	assert.Equal(t, len(values2test_domains()), len(records))

	provider.Client = &mockShamanClientFail{}
	_, err = provider.Domains(ctx)
	if err == nil {
		t.Errorf("expected to fail")
	}
}

func TestShamanRecords(t *testing.T) {
	provider := &ShamanProvider{
		Client: &mockShamanClient{},
	}
	ctx := context.Background()

	records, err := provider.Records(ctx)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	assert.Equal(t, len(values2test_records()), len(records))
	provider.Client = &mockShamanClientFail{}
	_, err = provider.Records(ctx)
	if err == nil {
		t.Errorf("expected to fail")
	}
}


func TestShamanApplyChanges(t *testing.T) {
	changes := &plan.Changes{}
	provider := &ShamanProvider{
		Client: &mockShamanClient{},
	}
	ctx := context.Background()

	changes.Create = []*endpoint.Endpoint{{DNSName: "new.ext-dns-test.3xmgroup.com.", Targets: endpoint.Targets{"target"}}, {DNSName: "new.ext-dns-test.unrelated.to.", Targets: endpoint.Targets{"target"}}}
	changes.Delete = []*endpoint.Endpoint{{DNSName: "foobar.ext-dns-test.3xmgroup.com.", Targets: endpoint.Targets{"target"}}}
	changes.UpdateOld = []*endpoint.Endpoint{{DNSName: "foobar.ext-dns-test.3xmgroup.com.", Targets: endpoint.Targets{"target-old"}}}
	changes.UpdateNew = []*endpoint.Endpoint{{DNSName: "foobar.ext-dns-test.3xmgroup.com.", Targets: endpoint.Targets{"target-new"}}}

	err := provider.ApplyChanges(ctx, changes)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	// empty changes
	changes.Create = []*endpoint.Endpoint{}
	changes.Delete = []*endpoint.Endpoint{}
	changes.UpdateOld = []*endpoint.Endpoint{}
	changes.UpdateNew = []*endpoint.Endpoint{}

	err = provider.ApplyChanges(ctx, changes)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

}
/* ********
 Test functions
************************************************/
