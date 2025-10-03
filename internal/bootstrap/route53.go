package bootstrap

import (
	koordeConfig "KoordeDHT/internal/config"
	"KoordeDHT/internal/domain"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

type Route53Bootstrap struct {
	client       *route53.Client
	hostedZoneID string
	domainSuffix string
	ttl          int64
}

func NewRoute53Bootstrap(cfg koordeConfig.Route53Config) (*Route53Bootstrap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := newClient(ctx)
	if err != nil {
		return nil, err
	}
	return &Route53Bootstrap{
		client:       client,
		hostedZoneID: cfg.HostedZoneID,
		domainSuffix: strings.TrimSuffix(cfg.DomainSuffix, "."),
		ttl:          cfg.TTL,
	}, nil
}

// newClient creates a new Route53 client using the default AWS config.
func newClient(ctx context.Context) (*route53.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return route53.NewFromConfig(awsCfg), nil
}

// Discover queries Route53 for SRV records in the specified hosted zone
func (r *Route53Bootstrap) Discover(ctx context.Context) ([]string, error) {
	// create a list to hold the discovered endpoints
	var endpoints []string
	// get the list of resource record sets in the hosted zone
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(r.hostedZoneID),
	}
	// Use a paginator to handle potentially large result sets
	paginator := route53.NewListResourceRecordSetsPaginator(r.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list records: %w", err)
		}
		// Process each record set in the page
		for _, rrset := range page.ResourceRecordSets {
			if rrset.Type != "SRV" {
				continue
			}
			if !strings.HasSuffix(strings.TrimSuffix(*rrset.Name, "."), r.domainSuffix) {
				continue
			}

			for _, rr := range rrset.ResourceRecords {
				var prio, weight, port int
				var target string
				_, err := fmt.Sscanf(*rr.Value, "%d %d %d %s", &prio, &weight, &port, &target)
				if err != nil {
					continue
				}
				target = strings.TrimSuffix(target, ".")

				ips, err := net.LookupHost(target)
				if err != nil {
					continue
				}
				for _, ip := range ips {
					endpoints = append(endpoints, fmt.Sprintf("%s:%d", ip, port))
				}
			}
		}
	}

	return endpoints, nil
}

// Register creates or updates an SRV record in Route53 for the given node.
func (r *Route53Bootstrap) Register(ctx context.Context, node *domain.Node) error {
	// create the full record name
	recordName := fmt.Sprintf("%s.%s.", node.ID.ToHexString(true), r.domainSuffix)
	// Extract host and port from node.Addr
	host, port, err := net.SplitHostPort(node.Addr)
	if err != nil {
		return err
	}
	// Insert the record into Route53
	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.hostedZoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(recordName),
						Type: types.RRTypeSrv,
						TTL:  aws.Int64(r.ttl),
						ResourceRecords: []types.ResourceRecord{
							{
								// Format: priority weight port target (priority and weight set to 0)
								Value: aws.String(fmt.Sprintf("0 0 %d %s.", port, host)),
							},
						},
					},
				},
			},
		},
	}
	_, err = r.client.ChangeResourceRecordSets(ctx, input)
	return err
}

// Deregister removes the SRV record for the given node from Route53.
func (r *Route53Bootstrap) Deregister(ctx context.Context, node *domain.Node) error {
	// create the full record name
	recordName := fmt.Sprintf("%s.%s.", node.ID.ToHexString(true), r.domainSuffix)
	// Extract host and port from node.Addr
	host, port, err := net.SplitHostPort(node.Addr)
	if err != nil {
		return err
	}
	// remove the record from Route53
	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.hostedZoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionDelete,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(recordName),
						Type: types.RRTypeSrv,
						TTL:  aws.Int64(r.ttl),
						ResourceRecords: []types.ResourceRecord{
							{
								Value: aws.String(fmt.Sprintf("0 0 %d %s.", port, host)),
							},
						},
					},
				},
			},
		},
	}
	_, err = r.client.ChangeResourceRecordSets(ctx, input)
	return err
}
