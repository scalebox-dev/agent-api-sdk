package agentapi

import (
	"context"
	"net/url"
)

type ResponsesService struct {
	http *httpClient
	path string
}

type ListResponsesParams struct {
	Limit            int
	PageToken        string
	SafetyIdentifier string
	UserID           string
}

type RetrieveResponseParams struct {
	SafetyIdentifier string
}

type ListEventsParams struct {
	AfterSequence int64
	View          string
}

func (s *ResponsesService) Create(ctx context.Context, params ResponseCreateParams, opts ...RequestOption) (*AgentResponse, error) {
	params.Stream = false
	if err := validateUniqueToolNames(params.Tools); err != nil {
		return nil, err
	}
	var out AgentResponse
	if err := s.http.requestJSON(ctx, "POST", s.path, params, &out, opts...); err != nil {
		return nil, err
	}
	addOutputText(&out)
	return &out, nil
}

func (s *ResponsesService) CreateStream(ctx context.Context, params ResponseCreateParams, opts ...RequestOption) (*ResponseStream, error) {
	params.Stream = true
	if err := validateUniqueToolNames(params.Tools); err != nil {
		return nil, err
	}
	resp, err := s.http.stream(ctx, "POST", s.path, params, opts...)
	if err != nil {
		return nil, err
	}
	return newResponseStream(resp), nil
}

func (s *ResponsesService) List(ctx context.Context, params ListResponsesParams, opts ...RequestOption) (*ListResponsesResponse, error) {
	var out ListResponsesResponse
	err := s.http.requestJSON(ctx, "GET", s.path+buildQuery(map[string]any{
		"limit":             params.Limit,
		"page_token":        params.PageToken,
		"safety_identifier": params.SafetyIdentifier,
		"user_id":           params.UserID,
	}), nil, &out, opts...)
	return &out, err
}

func (s *ResponsesService) Retrieve(ctx context.Context, responseID string, opts ...RequestOption) (*AgentResponse, error) {
	return s.RetrieveWithParams(ctx, responseID, RetrieveResponseParams{}, opts...)
}

func (s *ResponsesService) RetrieveWithParams(ctx context.Context, responseID string, params RetrieveResponseParams, opts ...RequestOption) (*AgentResponse, error) {
	var out AgentResponse
	err := s.http.requestJSON(ctx, "GET", s.path+"/"+url.PathEscape(responseID)+buildQuery(map[string]any{
		"safety_identifier": params.SafetyIdentifier,
	}), nil, &out, opts...)
	if err != nil {
		return nil, err
	}
	addOutputText(&out)
	return &out, nil
}

func (s *ResponsesService) Cancel(ctx context.Context, responseID string, opts ...RequestOption) (*CancelResponse, error) {
	var out CancelResponse
	err := s.http.requestJSON(ctx, "POST", s.path+"/"+url.PathEscape(responseID)+"/cancel", nil, &out, opts...)
	return &out, err
}

func (s *ResponsesService) ListChildren(ctx context.Context, responseID string, opts ...RequestOption) (*ListChildrenResponse, error) {
	var out ListChildrenResponse
	err := s.http.requestJSON(ctx, "GET", s.path+"/"+url.PathEscape(responseID)+"/children", nil, &out, opts...)
	return &out, err
}

func (s *ResponsesService) ListEvents(ctx context.Context, responseID string, params ListEventsParams, opts ...RequestOption) (*ListEventsResponse, error) {
	var out ListEventsResponse
	err := s.http.requestJSON(ctx, "GET", s.path+"/"+url.PathEscape(responseID)+"/events"+buildQuery(map[string]any{
		"after_sequence": params.AfterSequence,
		"view":           params.View,
	}), nil, &out, opts...)
	return &out, err
}

func (s *ResponsesService) RetrieveVolume(ctx context.Context, responseID string, opts ...RequestOption) (*VolumeInfo, error) {
	var out VolumeInfo
	err := s.http.requestJSON(ctx, "GET", s.path+"/"+url.PathEscape(responseID)+"/volume", nil, &out, opts...)
	return &out, err
}

func addOutputText(resp *AgentResponse) {
	if resp == nil || resp.OutputText != "" {
		return
	}
	var text string
	for _, item := range resp.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				text += part.Text
			}
		}
	}
	resp.OutputText = text
}
