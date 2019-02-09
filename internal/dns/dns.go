package dns

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
)

type DNSProvider struct {
	dnsClient *dns.Service
	projectID string
	zone      *dns.ManagedZone
}

func New(domain string) (*DNSProvider, error) {
	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, dns.CloudPlatformScope, dns.NdevClouddnsReadwriteScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create token source")
	}

	client := oauth2.NewClient(ctx, ts)
	dnsClient, err := dns.New(client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Cloud DNS client")
	}

	credentials, err := google.FindDefaultCredentials(ctx, dns.CloudPlatformScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get default credentials")
	}

	fmt.Printf("Current project: %s\n", credentials.ProjectID)

	p := &DNSProvider{
		dnsClient: dnsClient,
		projectID: credentials.ProjectID,
	}

	p.zone, err = p.findRelevantZone(domain)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find relevant zone")
	}

	return p, nil
}

func (d *DNSProvider) findRelevantZone(name string) (*dns.ManagedZone, error) {
	var relevantZone *dns.ManagedZone

	zonesCall := d.dnsClient.ManagedZones.List(d.projectID)
	zones, err := zonesCall.Do()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list zones")
	}
	for _, zone := range zones.ManagedZones {
		if strings.HasSuffix(name, zone.DnsName) || strings.HasSuffix(fmt.Sprintf("%s.", name), zone.DnsName) {
			if relevantZone == nil {
				relevantZone = zone
			} else {
				if len(zone.DnsName) > len(relevantZone.DnsName) {
					relevantZone = zone
				}
			}
		}
	}

	return relevantZone, nil
}

func (d *DNSProvider) GetRRs(name string) ([]*dns.ResourceRecordSet, error) {
	rrlc := d.dnsClient.ResourceRecordSets.List(d.projectID, d.zone.Name)
	rrl, err := rrlc.Name(name).Do()
	if err != nil {
		return nil, err
	}

	return rrl.Rrsets, nil
}

func (d *DNSProvider) executeChange(changeSet *dns.Change) error {
	change := d.dnsClient.Changes.Create(d.projectID, d.zone.Name, changeSet)

	if _, err := change.Do(); err != nil {
		return errors.Wrap(err, "failed to update DNS")
	}

	return nil
}

func (d *DNSProvider) Update(name, recordType, value string, ttl int64) error {
	fmt.Printf("dns.Update(%s, %s, %s, %d)\n", name, recordType, value, ttl)

	rrs, err := d.GetRRs(name)
	if err != nil {
		return errors.Wrap(err, "failed to lookup existing RRs")
	}
	var existingRR *dns.ResourceRecordSet
	for _, rr := range rrs {
		if rr.Type == recordType {
			fmt.Printf("Existing record: %s %s -> %s\n", rr.Type, rr.Name, rr.Rrdatas[0])
			existingRR = rr
		}
	}

	var changeRR []*dns.ResourceRecordSet
	changeRR = append(changeRR, &dns.ResourceRecordSet{
		Name:    name,
		Rrdatas: []string{value},
		Type:    recordType,
		Ttl:     ttl,
	})
	c := dns.Change{
		Additions: changeRR,
	}
	if existingRR != nil {
		c.Deletions = append(c.Deletions, existingRR)
	}

	return d.executeChange(&c)
}
