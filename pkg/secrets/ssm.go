// AWS Systems Manager Parameter Store source.
//
// This file ships as a stub to keep the repo lean — wiring the AWS SDK
// pulls ~25 MB of transitive dependencies. To enable:
//
//   go get github.com/aws/aws-sdk-go-v2/config@latest \
//          github.com/aws/aws-sdk-go-v2/service/ssm@latest
//
// Then replace the stub body with:
//
//   import (
//       awsconfig "github.com/aws/aws-sdk-go-v2/config"
//       "github.com/aws/aws-sdk-go-v2/service/ssm"
//   )
//
//   type SSMSource struct { client *ssm.Client; prefix string }
//
//   func NewSSMSource(ctx context.Context) (*SSMSource, error) {
//       cfg, err := awsconfig.LoadDefaultConfig(ctx)
//       if err != nil { return nil, err }
//       return &SSMSource{
//           client: ssm.NewFromConfig(cfg),
//           prefix: os.Getenv("SSM_PARAMETER_PREFIX"), // e.g. "/hamsaya/prod/"
//       }, nil
//   }
//
//   func (s *SSMSource) Get(ctx context.Context, key string) (string, error) {
//       out, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{
//           Name:           aws.String(s.prefix + key),
//           WithDecryption: aws.Bool(true),
//       })
//       if err != nil { return "", err }
//       if out.Parameter == nil || out.Parameter.Value == nil { return "", nil }
//       return *out.Parameter.Value, nil
//   }
//
// IAM policy needed:
//   ssm:GetParameter, ssm:GetParameters on arn:aws:ssm:*:*:parameter/<prefix>*
//   kms:Decrypt if SecureString parameters use a customer-managed CMK
//
// Rotation: SSM updates are immediate. CachingSource at the call site
// gives a 5-minute window before new values propagate (acceptable for
// secrets that rotate weekly or monthly).

package secrets

import "context"

// SSMSource is a stub. Real impl loads AWS SDK + GetParameter API.
// Returns ErrSourceNotConfigured until wired (see file header).
type SSMSource struct{}

// NewSSMSource returns ErrSourceNotConfigured until the AWS SDK is wired.
func NewSSMSource(_ context.Context) (*SSMSource, error) {
	return nil, ErrSourceNotConfigured
}

// Get is unreachable (constructor errors first). Defined to satisfy Source.
func (*SSMSource) Get(_ context.Context, _ string) (string, error) {
	return "", ErrSourceNotConfigured
}
