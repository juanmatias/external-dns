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

package shaman

import (
    "context"
    "os"
    "fmt"

    shaman "gitlab.com/tooling2/shaman-client-go"
    log "github.com/sirupsen/logrus"

    "sigs.k8s.io/external-dns/endpoint"
    "sigs.k8s.io/external-dns/plan"
    "sigs.k8s.io/external-dns/provider"
)

/* *******************************************
 Constants, structs and interfaces
*/

const (
    defaultShamanRecordTTL = 60
    defaultShamanRecordClass = "IN"
    shamanCreate = "CREATE"
    shamanDelete = "DELETE"
    shamanUpdate = "UPDATE"

)

type shamanDNS interface {
    GetDomains() (shaman.DomainsResponse, error)
    GetRecords(domainName string) (shaman.DomainResponse, error)
    DeleteDomain(domainName string) (shaman.MsgResponse, error)
    CreateDomain(name string) (shaman.DomainResponse, error)
    CreateRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error)
    CreateDomainWithRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error)
    ReplaceDomains(domainNames []string) ([]shaman.DomainResponse, error)
    ReplaceDomainsWithRecords(replacementDomains []shaman.Domain) ([]shaman.DomainResponse, error)
    ReplaceDomainRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error)
}

// ShamanProvider is an implementation of Provider for Shaman DNS.
type ShamanProvider struct {
    provider.BaseProvider
    Client shamanDNS
    // only consider hosted zones managing domains ending in this suffix
    domainFilter      endpoint.DomainFilter
    DryRun            bool
}

type shamanService struct {
    service *shaman.API
}

// cloudFlareChange differentiates between ChangActions
type shamanChangeSet struct {
    Action            string
    ResourceRecordSet shaman.Domain
}

// cloudFlareChange differentiates between ChangActions
type shamanChange struct {
    Domain      string
    Changes     []*shamanChangeSet
}

/* 
 Constants, structs and interfaces
******************************************* */


/* *******************************************
 The following two methods are the ones 
 required by Provider interface
*/
func (p *ShamanProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
    domains, err := p.Domains(ctx)
    if err != nil {
        return nil, err
    }

    endpoints := []*endpoint.Endpoint{}
    for _, domain := range domains {
        records, err := p.Client.GetRecords(domain)
        if err != nil {
            return nil, err
        }

        endpoints = append(endpoints, shamanGroupByNameAndType(records)...)
    }

    return endpoints, nil
}

// ApplyChanges applies a given set of changes in a given zone.
func (p *ShamanProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {

    combinedChanges := newShamanChanges(changes.Create, changes.UpdateNew, changes.Delete)

    return p.submitChanges(ctx, combinedChanges)
}

/* 
 The following two methods are the ones 
 required by Provider interface
******************************************* */


/* *******************************************
 Client provisioner functions
*/

// NewShamanProvider initializes a new Shaman DNS based Provider.
func NewShamanProvider(domainFilter endpoint.DomainFilter, dryRun bool) (*ShamanProvider, error) {
	// initialize via chosen auth method and returns new API object
	var (
		config *shaman.API
		err    error
	)
	if os.Getenv("SHAMAN_API_TOKEN") != "" {
        baseURL := ""
	    if os.Getenv("SHAMAN_BASE_URL") != "" {
            baseURL = os.Getenv("SHAMAN_BASE_URL")
        }
		config, err = shaman.NewWithAPIToken(os.Getenv("SHAMAN_API_TOKEN"), baseURL)
	}else{
		return nil, fmt.Errorf("No api token provided")
    }
	if err != nil {
		return nil, fmt.Errorf("failed to initialize shaman provider: %v", err)
	}
	provider := &ShamanProvider{
		Client:           shamanService{config},
		domainFilter:     domainFilter,
		DryRun:           dryRun,
	}
	return provider, nil
}
/* 
 Client provisioner functions
******************************************* */


/* *******************************************
 Provider functions
*/

// submitChanges takes a zone and a collection of Changes and sends them as a single transaction.
func (p *ShamanProvider) submitChanges(ctx context.Context, changes []*shamanChange) error {
    // return early if there is nothing to change
    if len(changes) == 0 {
        return nil
    }

    domains, err := p.Domains(ctx)
    if err != nil {
        return err
    }

    plannedDomains := []shaman.Domain{}

    for _, c := range changes {
        plannedRecords := []shaman.DNSRecord{}

        // search the domain in domains
        found := 0
        for _, d := range domains {
            if d == c.Domain {
                found = 1
            }
        }
        if found == 1 {
            // if exists get records
            records, err := p.Client.GetRecords(c.Domain)
            if err != nil {
                return err
            }
            for _, record := range records.Records {
                plannedRecords = append(plannedRecords, shaman.DNSRecord{
                                    TTL: record.TTL,
                                    Class: record.Class,
                                    Type: record.Type,
                                    Address: record.Address, 
                                })
            }
        }
        // evaluate required changes
            // get CREATE changes for domain
            currentChangeSet := &shamanChangeSet{}
            for _, changeset := range c.Changes {
                if changeset.Action == shamanCreate {
                    currentChangeSet = changeset
                    break
                }
            }
            // if records to create do not exist, then add them to plan
            for _, ccs := range currentChangeSet.ResourceRecordSet.Records {
                proceed := true
                for _, pr := range plannedRecords {
                    if sameShamanDNSRecord(pr, ccs){
                        logFields := log.Fields{
                            "domain":  c.Domain,
                            "type":    ccs.Type,
                            "ttl":     ccs.TTL,
                            "class":   ccs.Class,
                            "address": ccs.Address,
                            "action":  shamanCreate,
                        }
                        log.WithFields(logFields).Errorf("Record requested to be created but already exists.")
                        proceed = false
                    }
                }
                if proceed {
                    logFields := log.Fields{
                        "domain":  c.Domain,
                        "type":    ccs.Type,
                        "ttl":     ccs.TTL,
                        "class":   ccs.Class,
                        "address": ccs.Address,
                        "action":  shamanCreate,
                    }
                    log.WithFields(logFields).Info("Record requested to be created.")
                    plannedRecords = append(plannedRecords, ccs)
                }
            }

            // get UPDATE changes for domain
            currentChangeSet = &shamanChangeSet{}
            for _, changeset := range c.Changes {
                if changeset.Action == shamanUpdate {
                    currentChangeSet = changeset
                    break
                }
            }
            // if records to update exist, then modify them in the plan
            for _, ccs := range currentChangeSet.ResourceRecordSet.Records {
                found := false
                for i , pr := range plannedRecords {
                    if sameShamanDNSRecord(pr, ccs){
                        logFields := log.Fields{
                            "domain":  c.Domain,
                            "type":    ccs.Type,
                            "ttl":     ccs.TTL,
                            "class":   ccs.Class,
                            "address": ccs.Address,
                            "action":  shamanUpdate,
                        }
                        log.WithFields(logFields).Info("Record requested to be modified.")
                        plannedRecords[i].Type = ccs.Type
                        plannedRecords[i].TTL = ccs.TTL
                        plannedRecords[i].Class = ccs.Class
                        plannedRecords[i].Address = ccs.Address
                        found = true
                    }
                }
                if ! found {
                    logFields := log.Fields{
                        "domain":  c.Domain,
                        "type":    ccs.Type,
                        "ttl":     ccs.TTL,
                        "class":   ccs.Class,
                        "address": ccs.Address,
                        "action":  shamanUpdate,
                    }
                    log.WithFields(logFields).Errorf("Record requested to be modified but it does not exist.")
                }
            }

            // get DELETE changes for domain
            currentChangeSet = &shamanChangeSet{}
            for _, changeset := range c.Changes {
                if changeset.Action == shamanDelete {
                    currentChangeSet = changeset
                    break
                }
            }
            // if records to update exist, then modify them in the plan
            for _, ccs := range currentChangeSet.ResourceRecordSet.Records {
                idx := -1
                for i , pr := range plannedRecords {
                    if sameShamanDNSRecord(pr, ccs){
                        idx = i 
                    }
                }
                if idx == -1 {
                    logFields := log.Fields{
                        "domain":  c.Domain,
                        "type":    ccs.Type,
                        "ttl":     ccs.TTL,
                        "class":   ccs.Class,
                        "address": ccs.Address,
                        "action":  shamanDelete,
                    }
                    log.WithFields(logFields).Errorf("Record requested to be deleted but it does not exist.")
                }else{
                    logFields := log.Fields{
                        "domain":  c.Domain,
                        "type":    ccs.Type,
                        "ttl":     ccs.TTL,
                        "class":   ccs.Class,
                        "address": ccs.Address,
                        "action":  shamanDelete,
                    }
                    log.WithFields(logFields).Info("Record requested to be deleted.")
                    plannedRecords = append(plannedRecords[:idx], plannedRecords[idx+1:]...)
                }
            }
        plannedDomains = append(plannedDomains, shaman.Domain{
                                        Domain: c.Domain,
                                        Records: plannedRecords,
                                })
    }
    // apply plan

    if p.DryRun {
        return nil
    }

    for _, pd := range plannedDomains {
        if len(pd.Records) > 0 {
            _, err = p.Client.ReplaceDomainRecords(pd.Domain, pd.Records)
            if err != nil {
                log.Errorf("Mr Clouseau, we had an error with domain %v: %v", pd.Domain, err)
            }else{
                logFields := log.Fields{
                    "domain": pd.Domain,
                    "records":  pd.Records,
                }
                log.WithFields(logFields).Debug("Record set applied to domain.")
            }
        }else{
            _, err = p.Client.DeleteDomain(pd.Domain)
            if err != nil {
                log.Errorf("Mr Kato, we had an error with domain %v: %v", pd.Domain, err)
            }else{
                logFields := log.Fields{
                    "domain": pd.Domain,
                    "records":  pd.Records,
                }
                log.WithFields(logFields).Debug("Domain delete due to empty recordset.")
            }
        }
    }

    return nil

}



// Zones returns the list of hosted zones.
func (p *ShamanProvider) Domains(ctx context.Context) (shaman.DomainsResponse, error) {
    result := shaman.DomainsResponse{}

    res, err := p.Client.GetDomains()
    if err != nil {
        return shaman.DomainsResponse{}, err
    }

    for _, d := range res {
        if !p.domainFilter.Match(d) {
            continue
        }

        result = append(result, d)

    }
    return result, nil
}

/* 
 Provider functions
******************************************* */

/* *******************************************
 Aux functions
*/
func sameShamanDNSRecord(left, right shaman.DNSRecord) bool {
    if left.Class == right.Class && left.Type == right.Type && left.Address == right.Address {
        return true
    }
    return false
}

func shamanGroupByNameAndType(domainRecords shaman.DomainResponse) []*endpoint.Endpoint {
    endpoints := []*endpoint.Endpoint{}

    // group supported records by name and type
    groups := map[string][]shaman.DNSRecordResponse{}

    for _, r := range domainRecords.Records {
        if !provider.SupportedRecordType(r.Type) {
            continue
        }

        groupBy := domainRecords.Domain + r.Type
        if _, ok := groups[groupBy]; !ok {
            groups[groupBy] = []shaman.DNSRecordResponse{}
        }

        groups[groupBy] = append(groups[groupBy], r)
    }

    // create single endpoint with all the targets for each name/type
    for _, records := range groups {
        targets := make([]string, len(records))
        for i, record := range records {
            targets[i] = record.Address
        }
        endpoints = append(endpoints,
            endpoint.NewEndpointWithTTL(
                domainRecords.Domain,
                records[0].Type,
                endpoint.TTL(uint8(records[0].TTL)),
                targets...))
    }

    return endpoints
}

// newShamanChanges returns a collection of Changes based on the given records and action.
func newShamanChanges(endpointsCreate []*endpoint.Endpoint, endpointsUpdate []*endpoint.Endpoint, endpointsDelete []*endpoint.Endpoint) []*shamanChange {

    changes := []*shamanChangeSet{}
    changesResponse := make([]*shamanChange, 0, len(endpointsCreate) + len(endpointsUpdate) + len(endpointsDelete) )
    groups := map[string]map[string][]shaman.DNSRecord{}

    allendpoints := map[string][]*endpoint.Endpoint{
        shamanCreate: endpointsCreate,
        shamanUpdate: endpointsUpdate,
        shamanDelete: endpointsDelete,
        }

    for action, endpoints := range allendpoints {
        for _, endpoint := range endpoints {
            ttl := defaultShamanRecordTTL
            if endpoint.RecordTTL.IsConfigured() {
                ttl = int(endpoint.RecordTTL)
            }

            groupBy := endpoint.DNSName
            if _, ok := groups[groupBy]; !ok {
                groups[groupBy] = map[string][]shaman.DNSRecord{
                                    shamanCreate: []shaman.DNSRecord{},
                                    shamanUpdate: []shaman.DNSRecord{},
                                    shamanDelete: []shaman.DNSRecord{},
                }
            }

            resourceRecordSet := make([]shaman.DNSRecord, len(endpoint.Targets))

            for i := range endpoint.Targets {
                resourceRecordSet[i] = shaman.DNSRecord{
                    TTL:     uint8(ttl),
                    Type:    endpoint.RecordType,
                    Address: endpoint.Targets[i],
                    Class:   defaultShamanRecordClass,
                }
            }
            groups[groupBy][action] = append(groups[groupBy][action],resourceRecordSet...)

        }
    }

    for d, v := range groups {
        changeSet := []*shamanChangeSet{}
        for a, vv := range v {
            change := shaman.Domain{
                        Domain:     d,
                        Records:    vv,
                    }
            changes = append(changes, &shamanChangeSet{
                            Action:             a,
                            ResourceRecordSet:  change,
                        })
        }
        changeSet = append(changeSet, changes...)
        domainChange := &shamanChange{
                        Domain: d,
                        Changes: changeSet,
                     }
        changesResponse = append(changesResponse, domainChange)
    }

    return changesResponse
}

/* 
 Aux functions
******************************************* */

/* *******************************************
 Service Provider functions
*/

func ( s shamanService ) GetDomains() (shaman.DomainsResponse, error) {
    return s.service.GetDomains()
}

func ( s shamanService ) GetRecords(domainName string) (shaman.DomainResponse, error) {
    return s.service.GetRecords(domainName)
}

func ( s shamanService ) DeleteDomain(domainName string) (shaman.MsgResponse, error) {
    return s.service.DeleteDomain(domainName)
}

func ( s shamanService ) CreateDomain(name string) (shaman.DomainResponse, error) {
    return s.service.CreateDomain(name)
}

func ( s shamanService ) CreateRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return s.service.CreateRecords(domainName, records)
}

func ( s shamanService ) CreateDomainWithRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return s.service.CreateDomainWithRecords(domainName, records)
}

func ( s shamanService ) ReplaceDomains(domainNames []string) ([]shaman.DomainResponse, error) {
    return s.service.ReplaceDomains(domainNames)
}

func ( s shamanService ) ReplaceDomainsWithRecords(replacementDomains []shaman.Domain) ([]shaman.DomainResponse, error) {
    return s.service.ReplaceDomainsWithRecords(replacementDomains)
}

func ( s shamanService ) ReplaceDomainRecords(domainName string, records []shaman.DNSRecord) (shaman.DomainResponse, error) {
    return s.service.ReplaceDomainRecords(domainName, records)
}

