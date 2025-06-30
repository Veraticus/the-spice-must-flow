# AI Analysis LLM Integration - Phase 0

**Critical Issue**: The current LLM client interface is designed specifically for transaction classification, not for general AI analysis. This needs to be addressed before the analysis feature can work properly.

## The Problem

The existing `llm.Client` interface has methods like:
- `Classify()` - Returns structured classification data
- `ClassifyWithRankings()` - Returns category rankings
- `GenerateDescription()` - Returns a description

The analysis feature needs to:
- Send complex prompts with transaction data, patterns, and categories
- Receive arbitrary JSON responses with analysis results
- Handle much larger responses than simple classifications

## Proposed Solution

### Option 1: Extend the LLM Client Interface (Recommended)

Add a new method to the `llm.Client` interface specifically for analysis:

```go
// internal/llm/client.go
type Client interface {
    // Existing methods...
    Classify(ctx context.Context, prompt string) (ClassificationResponse, error)
    ClassifyWithRankings(ctx context.Context, prompt string) (RankingResponse, error)
    ClassifyMerchantBatch(ctx context.Context, prompt string) (MerchantBatchResponse, error)
    GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error)
    
    // New method for analysis
    Analyze(ctx context.Context, prompt string, systemPrompt string) (string, error)
}
```

Implementation for each provider:

```go
// internal/llm/openai.go
func (c *openAIClient) Analyze(ctx context.Context, prompt string, systemPrompt string) (string, error) {
    messages := []openai.ChatCompletionMessage{
        {
            Role:    openai.ChatMessageRoleSystem,
            Content: systemPrompt,
        },
        {
            Role:    openai.ChatMessageRoleUser,
            Content: prompt,
        },
    }
    
    // Call OpenAI API and return raw response
    // No JSON parsing, just return the string
}
```

### Option 2: Create a Separate Analysis Client

Create a completely separate client interface for analysis:

```go
// internal/llm/analysis_client.go
type AnalysisClient interface {
    PerformAnalysis(ctx context.Context, request AnalysisRequest) (string, error)
}

type AnalysisRequest struct {
    SystemPrompt string
    UserPrompt   string
    Temperature  float64
    MaxTokens    int
}
```

### Option 3: Refactor LLMAnalysisAdapter

Make the `LLMAnalysisAdapter` work with the existing interface by:
- Using `GenerateDescription()` for simple analysis
- Breaking complex analysis into multiple classification calls
- Post-processing and combining results

## Implementation Steps

### For Option 1 (Recommended):

1. **Update the Interface** (30 minutes)
   - Add `Analyze()` method to `llm.Client`
   - Update all mock implementations

2. **Implement for Each Provider** (2 hours)
   - OpenAI: Direct API call without classification parsing
   - Anthropic: Similar implementation
   - ClaudeCode: Pass through to CLI

3. **Update LLMAnalysisAdapter** (1 hour)
   - Change to use the new `Analyze()` method
   - Remove the hack of using `Category` field for JSON

4. **Testing** (1 hour)
   - Unit tests for each provider
   - Integration tests with mock responses

## Benefits of This Approach

1. **Clean Separation**: Analysis and classification are different concerns
2. **Proper JSON Handling**: Analysis can return arbitrary JSON without parsing
3. **Better Prompting**: Can use different system prompts for analysis
4. **Larger Responses**: Can handle multi-page analysis reports
5. **Future Flexibility**: Easy to add more analysis-specific features

## Migration Path

1. Add the new method to the interface
2. Implement it for all providers
3. Update `LLMAnalysisAdapter` to use the new method
4. Remove the mock client workaround
5. Test with real LLM providers

## Considerations

- **Token Limits**: Analysis responses can be large, need proper handling
- **Cost**: Analysis prompts are longer and more expensive
- **Rate Limiting**: Need appropriate retry logic
- **Streaming**: Consider streaming responses for better UX

## Next Steps

1. Get approval on the approach
2. Implement the interface changes
3. Update all provider implementations
4. Refactor the analysis adapter
5. Remove temporary mocks
6. Test end-to-end

This phase should be completed before Phase 2 (Prompt Builder and LLM Integration) in the main implementation plan.