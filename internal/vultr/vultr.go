/*
Copyright (C) 2021-2023, Kubefirst

This program is licensed under MIT.
See the LICENSE file for more details.
*/
package vultr

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/kubefirst/kubefirst-api/internal/dns"
	"github.com/rs/zerolog/log"
	"github.com/vultr/govultr/v3"
)

func (c *VultrConfiguration) TestDomainLiveness(domainName string) bool {
	vultrRecordName := "kubefirst-liveness"
	vultrRecordValue := "domain record propagated"

	vultrRecordConfig := &govultr.DomainRecordReq{
		Name:     vultrRecordName,
		Type:     "TXT",
		Data:     vultrRecordValue,
		TTL:      600,
		Priority: govultr.IntToIntPtr(100),
	}

	log.Info().Msgf("checking to see if record %s exists", domainName)
	log.Info().Msgf("domainName %s", domainName)

	//check for existing records
	records, err := c.GetDNSRecords(domainName)
	if err != nil {
		log.Error().Msgf("error getting vultr dns records for domain %s: %s", domainName, err)
		return false
	}

	for _, record := range records {
		if record.Type == "TXT" && record.Name == vultrRecordName {
			return true
		}
	}

	//create record if it does not exist
	_, _, err = c.Client.DomainRecord.Create(c.Context, domainName, vultrRecordConfig)
	if err != nil {
		log.Warn().Msgf("%s", err)
		return false
	}
	log.Info().Msg("domain record created")

	count := 0
	// todo need to exit after n number of minutes and tell them to check ns records
	// todo this logic sucks
	for count <= 100 {
		count++

		log.Info().Msgf("%s", vultrRecordName)
		ips, err := net.LookupTXT(fmt.Sprintf("%s.%s", vultrRecordName, domainName))
		if err != nil {
			ips, err = dns.BackupResolver.LookupTXT(context.Background(), vultrRecordName)
		}

		log.Info().Msgf("%s", ips)

		if err != nil {
			log.Warn().Msgf("Could not get record name %s - waiting 10 seconds and trying again: \nerror: %s", vultrRecordName, err)
			time.Sleep(10 * time.Second)
		} else {
			for _, ip := range ips {
				// todo check ip against route53RecordValue in some capacity so we can pivot the value for testing
				log.Info().Msgf("%s. in TXT record value: %s\n", vultrRecordName, ip)
				count = 101
			}
		}
		if count == 100 {
			log.Error().Msg("unable to resolve domain dns record. please check your domain registrar")
			return false
		}
	}
	return true
}

// GetStorageBuckets retrieves all Vultr object storage buckets
func (c *VultrConfiguration) GetDNSRecords(domainName string) ([]govultr.DomainRecord, error) {
	records, _, _, err := c.Client.DomainRecord.List(c.Context, domainName, &govultr.ListOptions{})
	if err != nil {
		log.Error().Msgf("error getting vultr dns records for domain %s: %s", domainName, err)
		return []govultr.DomainRecord{}, err
	}

	return records, nil
}

// GetDNSInfo determines whether or not a domain exists within Vultr
func (c *VultrConfiguration) GetDNSInfo(domainName string) (string, error) {
	log.Info().Msg("GetDNSInfo (working...)")

	vultrDNSDomain, _, err := c.Client.Domain.Get(c.Context, domainName)
	if err != nil {
		log.Info().Msg(err.Error())
		return "", err
	}

	return vultrDNSDomain.Domain, nil
}

// GetDomainApexContent determines whether or not a target domain features
// a host responding at zone apex
func GetDomainApexContent(domainName string) bool {
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	exists := false
	for _, proto := range []string{"http", "https"} {
		fqdn := fmt.Sprintf("%s://%s", proto, domainName)
		_, err := client.Get(fqdn)
		if err != nil {
			log.Warn().Msgf("domain %s has no apex content", fqdn)
		} else {
			log.Info().Msgf("domain %s has apex content", fqdn)
			exists = true
		}
	}

	return exists
}

// GetDNSDomains lists all available DNS domains
func (c *VultrConfiguration) GetDNSDomains() ([]string, error) {
	var domainList []string

	domains, _, _, err := c.Client.Domain.List(c.Context, &govultr.ListOptions{})
	if err != nil {
		return []string{}, err
	}

	for _, domain := range domains {
		domainList = append(domainList, domain.Domain)
	}

	return domainList, nil
}

// GetRegions lists all available regions
func (c *VultrConfiguration) GetRegions() ([]string, error) {
	var regionList []string

	regions, _, _, err := c.Client.Region.List(c.Context, &govultr.ListOptions{})
	if err != nil {
		return []string{}, err
	}

	for _, region := range regions {
		regionList = append(regionList, region.ID)
	}

	return regionList, nil
}

func (c *VultrConfiguration) ListInstances() ([]string, error) {

	// can pass empty string to list all plans for second arg to List
	plans, _, _, err := c.Client.Plan.List(c.Context, "", &govultr.ListOptions{
		Region: c.Region,
	})

	if err != nil {
		return nil, err
	}

	var planNames []string
	for _, plan := range plans {
		planNames = append(planNames, plan.ID)
	}

	return planNames, nil
}
