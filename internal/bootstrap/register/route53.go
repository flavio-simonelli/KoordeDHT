package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// RegisterBootstrap crea o aggiorna un record SRV in Route53
func RegisterBootstrap(ctx context.Context, hostedZoneID, domain, targetHost string, port int) error {
	// Carica configurazione AWS dal contesto (IAM Role, env vars, ecc.)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	client := route53.NewFromConfig(cfg)

	// Definiamo il record SRV
	record := &types.ResourceRecordSet{
		Name: aws.String(fmt.Sprintf("_koorde._tcp.%s", domain)),
		Type: types.RRTypeSrv,
		TTL:  aws.Int64(30), // TTL basso per aggiornamenti veloci
		ResourceRecords: []types.ResourceRecord{
			{
				Value: aws.String(fmt.Sprintf("0 5 %d %s.", port, targetHost)),
			},
		},
	}

	// ChangeBatch
	change := &route53.ChangeBatch{
		Changes: []types.Change{
			{
				Action:            types.ChangeActionUpsert,
				ResourceRecordSet: record,
			},
		},
	}

	// Chiamata API
	_, err = client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
		ChangeBatch:  change,
	})

	if err != nil {
		return fmt.Errorf("failed to register bootstrap SRV record: %w", err)
	}

	return nil
}
