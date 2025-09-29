package register

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// NewClient creates a new Route53 client using the default AWS config.
func NewClient(ctx context.Context) (*route53.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return route53.NewFromConfig(awsCfg), nil
}
