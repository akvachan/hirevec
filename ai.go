// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"uuid"
)

type EmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     uint16    `json:"index"`
}

type EmbeddingsResponse struct {
	Data   []EmbeddingData `json:"data"`
	Object string          `json:"object"`
}

type EmbeddingsRequest struct {
	Input          []string `json:"input"`
	Model          string   `json:"model"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
}

type AIConfig struct {
	LlamaCppRerankURL       string
	LlamaCppEmbeddingsURL   string
	LlamaCppAPIKey          string
	LlamaCppEmbeddingsModel string
	LlamaCppRerankerModel   string
	LlamaCppEmbeddingsDim   uint32
	UseEmbeddings           bool
	UseReranker             bool
}

type RerankRequest struct {
	Model string   `json:"model"`
	Query string   `json:"query"`
	Texts []string `json:"texts"`
}

type RerankResult struct {
	Index uint8   `json:"index"`
	Score float32 `json:"score"`
}

const (
	DefaultDailyRecommendations uint8  = 32  // Limits number of daily recommendations for candidates and recruiters (per position)
	DefaultTopPositions         uint8  = 128 // Limits embeddings results
	DefaultEmbeddingsBatchSize  uint8  = 64  // Limits number of embeddings that the embeddings job produces in parallel
	DefaultCandidatesBatchSize  uint16 = 128 // Limits number of candidates that the recommendation job processes in parallel
)

func CreateEmbeddings(c AIConfig, input []string) ([]EmbeddingData, error) {
	requestBody, err := json.Marshal(EmbeddingsRequest{
		Model:          c.LlamaCppEmbeddingsModel,
		Input:          input,
		EncodingFormat: "float",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embeddings request: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, c.LlamaCppEmbeddingsURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to init new request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.LlamaCppAPIKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("failed to close response body", "err", err)
		}
	}()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("embedding endpoint returned %d, but server failed to read response body: %w", response.StatusCode, err)
		}
		return nil, fmt.Errorf("embedding endpoint returned %d: %s", response.StatusCode, body)
	}

	var parsed EmbeddingsResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	if len(parsed.Data) != len(input) {
		return nil, fmt.Errorf("embedding count mismatch (got %d, expected %d)", len(parsed.Data), len(input))
	}

	return parsed.Data, nil
}

func CreateRerank(c AIConfig, query string, documents []string, documentIDs []uuid.UUID) ([]RerankResult, error) {
	requestBody, err := json.Marshal(RerankRequest{
		Model: c.LlamaCppRerankerModel,
		Query: query,
		Texts: documents,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rerank request: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, c.LlamaCppRerankURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to init new request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.LlamaCppAPIKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("failed to close response body", "err", err)
		}
	}()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("rerank endpoint returned %d, but server failed to read response body: %w", response.StatusCode, err)
		}
		return nil, fmt.Errorf("rerank endpoint returned %d: %s", response.StatusCode, body)
	}

	var results []RerankResult
	if err := json.NewDecoder(response.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	if len(results) != len(documentIDs) {
		return nil, fmt.Errorf("rerank count mismatch (got %d, expected %d)", len(results), len(documentIDs))
	}

	return results, nil
}

func (a *API) Rerank(c AIConfig, candidateID uuid.UUID, positionsIDs []uuid.UUID) ([]RerankResult, error) {
	if len(positionsIDs) == 0 {
		return nil, nil
	}

	candidate, err := a.Store.GetCandidate(candidateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get candidate: %w", err)
	}

	positionsTexts, err := a.Store.GetPositionsTexts(positionsIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get position texts: %w", err)
	}

	return CreateRerank(c, candidate.About, positionsTexts, positionsIDs)
}
