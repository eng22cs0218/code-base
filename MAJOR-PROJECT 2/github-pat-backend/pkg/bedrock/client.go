package bedrock

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/types"
	"github.com/google/uuid"
)

// GenerateRemediation takes K8s resource and findings context, sends it to AWS Bedrock Agent,
// and returns the AI-generated remediation command.
func GenerateRemediation(promptContext string) (string, error) {
	ctx := context.TODO()

	// Load AWS config
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-south-1" // Defaulting to the region given in prompt
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			"",
		)),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config for Bedrock: %w", err)
	}

	client := bedrockagentruntime.NewFromConfig(cfg)

	// User specifically requested the agent E2NJYUZIH7
	agentID := "E2NJYUZIH7"
	aliasID := "TSTALIASID"

	sessionID := uuid.New().String()

	input := &bedrockagentruntime.InvokeAgentInput{
		AgentId:      aws.String(agentID),
		AgentAliasId: aws.String(aliasID),
		SessionId:    aws.String(sessionID),
		InputText:    aws.String(promptContext),
	}

	out, err := client.InvokeAgent(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to invoke Bedrock agent: %w", err)
	}

	// Read event stream response
	var responseText string

	for event := range out.GetStream().Events() {
		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			responseText += string(v.Value.Bytes)
		case *types.UnknownUnionMember:
			fmt.Println("unknown tag:", v.Tag)
		}
	}

	if err := out.GetStream().Err(); err != nil {
		return "", fmt.Errorf("error reading response stream: %w", err)
	}

	return responseText, nil
}
