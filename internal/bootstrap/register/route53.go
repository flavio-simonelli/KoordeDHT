// register/route53.go
package register

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

type Route53Registrar struct {
	Client       *route53.Client
	HostedZoneID string
	DomainSuffix string
	TTL          int64
}

// NewRoute53Registrar loads AWS config and returns a registrar.
func NewRoute53Registrar(ctx context.Context, hostedZoneID, domainSuffix string, ttl int64) (*Route53Registrar, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &Route53Registrar{
		Client:       route53.NewFromConfig(awsCfg),
		HostedZoneID: hostedZoneID,
		DomainSuffix: strings.TrimSuffix(domainSuffix, "."),
		TTL:          ttl,
	}, nil
}

func (r *Route53Registrar) RegisterNode(ctx context.Context, nodeID, targetHost string, port int) error {
	recordName := fmt.Sprintf("%s.%s.", nodeID, r.DomainSuffix)
	if strings.HasSuffix(targetHost, ".") {
		targetHost = targetHost[:len(targetHost)-1]
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.HostedZoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(recordName),
						Type: types.RRTypeSrv,
						TTL:  aws.Int64(r.TTL),
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
	_, err := r.Client.ChangeResourceRecordSets(ctx, input)
	return err
}

func (r *Route53Registrar) DeregisterNode(ctx context.Context, nodeID, targetHost string, port int) error {
	recordName := fmt.Sprintf("%s.%s.", nodeID, r.DomainSuffix)
	if strings.HasSuffix(targetHost, ".") {
		targetHost = targetHost[:len(targetHost)-1]
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.HostedZoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionDelete,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(recordName),
						Type: types.RRTypeSrv,
						TTL:  aws.Int64(r.TTL),
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
	_, err := r.Client.ChangeResourceRecordSets(ctx, input)
	return err
}

func (r *Route53Registrar) RenewNode(ctx context.Context, nodeID, targetHost string, port int) error {
	// Route53 non ha bisogno di rinnovo, basta Upsert
	return nil
}

func (r *Route53Registrar) Close() error {
	// Non c'è nulla da chiudere
	return nil
}
