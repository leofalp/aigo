package duckduckgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/tool"
)

// flexibleInt is a private type that can unmarshal both string and int values
type flexibleInt string

func (f *flexibleInt) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int first
	var i int
	if err := json.Unmarshal(data, &i); err == nil {
		*f = flexibleInt(strconv.Itoa(i))
		return nil
	}

	// Try as string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexibleInt(s)
		return nil
	}

	// Default to empty string
	*f = ""
	return nil
}

func (f *flexibleInt) String() string {
	return string(*f)
}

// makeAbsoluteURL converts relative DuckDuckGo URLs to absolute URLs
func makeAbsoluteURL(urlPath string) string {
	if urlPath == "" {
		return ""
	}
	if strings.HasPrefix(urlPath, "http://") || strings.HasPrefix(urlPath, "https://") {
		return urlPath
	}
	if strings.HasPrefix(urlPath, "/") {
		return "https://duckduckgo.com" + urlPath
	}
	return urlPath
}

func NewDuckDuckGoSearchTool() *tool.Tool[Input, Output] {
	return tool.NewTool[Input, Output](
		"DuckDuckGoSearch",
		Search,
		tool.WithDescription("Search the web using DuckDuckGo search engine. Returns instant answers, abstracts, and related topics summary for a given query."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.0, // Free - DuckDuckGo Instant Answer API is free
			Currency:                "USD",
			CostDescription:         "free API",
			Accuracy:                0.50, // Good for instant answers, but less comprehensive than paid search APIs
			AverageDurationInMillis: 600,  // Average API response time (~600ms)
		}),
	)
}

func NewDuckDuckGoSearchAdvancedTool() *tool.Tool[Input, AdvancedOutput] {
	return tool.NewTool[Input, AdvancedOutput](
		"DuckDuckGoSearchAdvanced",
		SearchAdvanced,
		tool.WithDescription("Advanced web search using DuckDuckGo. Returns complete structured results including abstracts, answers, definitions, related topics with full metadata, and image information with absolute URLs."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.0, // Free - DuckDuckGo Instant Answer API is free
			Currency:                "USD",
			CostDescription:         "free API",
			Accuracy:                0.55, // Better accuracy with full structured data
			AverageDurationInMillis: 650,  // Slightly slower due to more data processing
		}),
	)
}

// fetchDDGResponse is the shared function that performs the API call
func fetchDDGResponse(ctx context.Context, query string) (*DDGResponse, error) {
	params := url.Values{}
	params.Add("q", query)
	params.Add("format", "json")
	params.Add("no_html", "1")
	params.Add("skip_disambig", "1")

	fullURL := "https://api.duckduckgo.com/?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	httpReq.Header.Set("User-Agent", "aigo-duckduckgo-tool/1.0")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer utils.CloseWithLog(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var ddgResponse DDGResponse
	if err := json.Unmarshal(body, &ddgResponse); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return &ddgResponse, nil
}

func Search(ctx context.Context, req Input) (Output, error) {
	ddgResponse, err := fetchDDGResponse(ctx, req.Query)
	if err != nil {
		return Output{}, err
	}

	// Build summary
	var results []string

	if ddgResponse.AbstractText != "" {
		results = append(results, fmt.Sprintf("Abstract: %s", ddgResponse.AbstractText))
		if ddgResponse.AbstractURL != "" {
			results = append(results, fmt.Sprintf("Source: %s", ddgResponse.AbstractURL))
		}
	}

	if ddgResponse.Answer != "" {
		results = append(results, fmt.Sprintf("Answer: %s", ddgResponse.Answer))
	}

	if ddgResponse.Definition != "" {
		results = append(results, fmt.Sprintf("Definition: %s", ddgResponse.Definition))
	}

	// Add related topics (limited to 5)
	if len(ddgResponse.RelatedTopics) > 0 {
		topics := []string{}
		for i, topic := range ddgResponse.RelatedTopics {
			if i >= 5 {
				break
			}
			if topic.Text != "" {
				topics = append(topics, topic.Text)
			}
		}
		if len(topics) > 0 {
			results = append(results, fmt.Sprintf("Related topics: %s", strings.Join(topics, "; ")))
		}
	}

	summary := strings.Join(results, "\n\n")
	if summary == "" {
		summary = "No results found for this query."
	}

	return Output{
		Query:   req.Query,
		Summary: summary,
	}, nil
}

func SearchAdvanced(ctx context.Context, req Input) (AdvancedOutput, error) {
	ddgResponse, err := fetchDDGResponse(ctx, req.Query)
	if err != nil {
		return AdvancedOutput{}, err
	}

	// Convert internal response types to public types
	relatedTopics := make([]RelatedTopic, len(ddgResponse.RelatedTopics))
	for i, rt := range ddgResponse.RelatedTopics {
		relatedTopics[i] = rt.toRelatedTopic()
	}

	results := make([]Result, len(ddgResponse.Results))
	for i, r := range ddgResponse.Results {
		results[i] = r.toResult()
	}

	return AdvancedOutput{
		Query:          req.Query,
		Abstract:       ddgResponse.AbstractText,
		AbstractSource: ddgResponse.AbstractSource,
		AbstractURL:    ddgResponse.AbstractURL,
		Answer:         ddgResponse.Answer,
		AnswerType:     ddgResponse.AnswerType,
		Definition:     ddgResponse.Definition,
		DefinitionURL:  ddgResponse.DefinitionURL,
		Heading:        ddgResponse.Heading,
		Image:          makeAbsoluteURL(ddgResponse.Image),
		ImageWidth:     ddgResponse.ImageWidth.String(),
		ImageHeight:    ddgResponse.ImageHeight.String(),
		ImageIsLogo:    ddgResponse.ImageIsLogo.String(),
		RelatedTopics:  relatedTopics,
		Results:        results,
		Type:           ddgResponse.Type,
		Redirect:       ddgResponse.Redirect,
	}, nil
}

type Input struct {
	Query string `json:"query" jsonschema:"description=The search query to look up on DuckDuckGo,required"`
}

type Output struct {
	Query   string `json:"query" jsonschema:"description=The original search query"`
	Summary string `json:"summary" jsonschema:"description=Summary of search results including abstracts, answers, and related topics"`
}

type AdvancedOutput struct {
	Query          string         `json:"query" jsonschema:"description=The original search query"`
	Abstract       string         `json:"abstract,omitempty" jsonschema:"description=Abstract text about the topic"`
	AbstractSource string         `json:"abstract_source,omitempty" jsonschema:"description=Source name for the abstract (e.g. Wikipedia)"`
	AbstractURL    string         `json:"abstract_url,omitempty" jsonschema:"description=URL to the source of the abstract"`
	Answer         string         `json:"answer,omitempty" jsonschema:"description=Instant answer for the query (calculations conversions etc)"`
	AnswerType     string         `json:"answer_type,omitempty" jsonschema:"description=Type of answer provided"`
	Definition     string         `json:"definition,omitempty" jsonschema:"description=Dictionary definition if available"`
	DefinitionURL  string         `json:"definition_url,omitempty" jsonschema:"description=URL to the definition source"`
	Heading        string         `json:"heading,omitempty" jsonschema:"description=Heading or title of the result"`
	Image          string         `json:"image,omitempty" jsonschema:"description=URL to a relevant image"`
	ImageWidth     string         `json:"image_width,omitempty" jsonschema:"description=Width of the image in pixels"`
	ImageHeight    string         `json:"image_height,omitempty" jsonschema:"description=Height of the image in pixels"`
	ImageIsLogo    string         `json:"image_is_logo,omitempty" jsonschema:"description=1 if image is a logo 0 otherwise"`
	RelatedTopics  []RelatedTopic `json:"related_topics,omitempty" jsonschema:"description=List of related topics with full metadata"`
	Results        []Result       `json:"results,omitempty" jsonschema:"description=Additional search results"`
	Type           string         `json:"type,omitempty" jsonschema:"description=Type of result (A=article C=category D=disambiguation E=exclusive N=nothing)"`
	Redirect       string         `json:"redirect,omitempty" jsonschema:"description=Redirect URL if applicable"`
}

// DDGResponse represents the DuckDuckGo API response (internal)
type DDGResponse struct {
	Abstract       string                 `json:"Abstract"`
	AbstractText   string                 `json:"AbstractText"`
	AbstractSource string                 `json:"AbstractSource"`
	AbstractURL    string                 `json:"AbstractURL"`
	Answer         string                 `json:"Answer"`
	AnswerType     string                 `json:"AnswerType"`
	Definition     string                 `json:"Definition"`
	DefinitionURL  string                 `json:"DefinitionURL"`
	Heading        string                 `json:"Heading"`
	Image          string                 `json:"Image"`
	ImageWidth     flexibleInt            `json:"ImageWidth"`
	ImageHeight    flexibleInt            `json:"ImageHeight"`
	ImageIsLogo    flexibleInt            `json:"ImageIsLogo"`
	RelatedTopics  []relatedTopicResponse `json:"RelatedTopics"`
	Results        []resultResponse       `json:"Results"`
	Type           string                 `json:"Type"`
	Redirect       string                 `json:"Redirect"`
}

type RelatedTopic struct {
	FirstURL string `json:"first_url" jsonschema:"description=URL to the related topic"`
	Icon     Icon   `json:"icon" jsonschema:"description=Icon information with absolute URL"`
	Result   string `json:"result" jsonschema:"description=HTML formatted result"`
	Text     string `json:"text" jsonschema:"description=Plain text description of the topic"`
}

type Result struct {
	FirstURL string `json:"first_url" jsonschema:"description=URL to the result"`
	Icon     Icon   `json:"icon" jsonschema:"description=Icon information with absolute URL"`
	Result   string `json:"result" jsonschema:"description=HTML formatted result"`
	Text     string `json:"text" jsonschema:"description=Plain text description of the result"`
}

// Internal types for JSON unmarshaling
type relatedTopicResponse struct {
	FirstURL string       `json:"FirstURL"`
	Icon     iconResponse `json:"Icon"`
	Result   string       `json:"Result"`
	Text     string       `json:"Text"`
}

type resultResponse struct {
	FirstURL string       `json:"FirstURL"`
	Icon     iconResponse `json:"Icon"`
	Result   string       `json:"Result"`
	Text     string       `json:"Text"`
}

func (r relatedTopicResponse) toRelatedTopic() RelatedTopic {
	return RelatedTopic{
		FirstURL: r.FirstURL,
		Icon:     r.Icon.toIcon(),
		Result:   r.Result,
		Text:     r.Text,
	}
}

func (r resultResponse) toResult() Result {
	return Result{
		FirstURL: r.FirstURL,
		Icon:     r.Icon.toIcon(),
		Result:   r.Result,
		Text:     r.Text,
	}
}

type Icon struct {
	URL    string `json:"url" jsonschema:"description=Icon URL (absolute)"`
	Height string `json:"height" jsonschema:"description=Icon height in pixels"`
	Width  string `json:"width" jsonschema:"description=Icon width in pixels"`
}

// iconResponse is used internally for JSON unmarshaling with flexible int handling
type iconResponse struct {
	URL    string      `json:"URL"`
	Height flexibleInt `json:"Height"`
	Width  flexibleInt `json:"Width"`
}

// toIcon converts iconResponse to Icon with string values
func (ir iconResponse) toIcon() Icon {
	return Icon{
		URL:    makeAbsoluteURL(ir.URL),
		Height: ir.Height.String(),
		Width:  ir.Width.String(),
	}
}
