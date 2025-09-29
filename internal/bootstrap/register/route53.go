package register

import (
	koordeConfig "KoordeDHT/internal/config"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// RegisterNode creates or updates an SRV record in Route53 for the given node.
func RegisterNode(ctx context.Context, client *route53.Client, cfg koordeConfig.RegisterConfig, nodeID string, targetHost string, port int) error {
	domain := strings.TrimSuffix(cfg.DomainSuffix, ".")
	recordName := fmt.Sprintf("%s.%s.", nodeID, domain)
	if strings.HasSuffix(targetHost, ".") {
		targetHost = targetHost[:len(targetHost)-1]
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(cfg.HostedZoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(recordName),
						Type: types.RRTypeSrv,
						TTL:  aws.Int64(cfg.TTL),
						ResourceRecords: []types.ResourceRecord{
							{
								// Format: priority weight port target
								Value: aws.String(fmt.Sprintf("0 0 %d %s.", port, targetHost)),
							},
						},
					},
				},
			},
		},
	}

	_, err := client.ChangeResourceRecordSets(ctx, input)
	return err
}

// DeregisterNode removes the SRV record for the given node from Route53.
func DeregisterNode(ctx context.Context, client *route53.Client, cfg koordeConfig.RegisterConfig, nodeID string, targetHost string, port int) error {
	domain := strings.TrimSuffix(cfg.DomainSuffix, ".")
	recordName := fmt.Sprintf("%s.%s.", nodeID, domain)
	if strings.HasSuffix(targetHost, ".") {
		targetHost = targetHost[:len(targetHost)-1]
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(cfg.HostedZoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionDelete,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(recordName),
						Type: types.RRTypeSrv,
						TTL:  aws.Int64(cfg.TTL),
						ResourceRecords: []types.ResourceRecord{
							{
								Value: aws.String(fmt.Sprintf("0 0 %d %s.", port, targetHost)),
							},
						},
					},
				},
			},
		},
	}

	_, err := client.ChangeResourceRecordSets(ctx, input)
	return err
}
