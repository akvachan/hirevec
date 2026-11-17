// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"fmt"
	"log/slog"
	"time"
	"uuid"
)

const (
	DefaultEmbeddingsJobFrequency      = 1 * time.Hour
	DefaultRecommendationsJobFrequency = 24 * time.Hour
	DefaultWebAuthnCleanupJobFrequency = 15 * time.Minute
	DefaultVisitorsCleanupJobFrequency = 1 * time.Minute
	DefaultVisitorsInactivityFrame     = 24 * time.Hour
)

func (a *API) RunWebAuthnCleanupJob() error {
	slog.Debug("running webauthn cleanup job")

	deleted, err := a.Store.DeleteExpiredWebAuthnSessions()
	if err != nil {
		return fmt.Errorf("failed to delete expired webauthn sessions: %w", err)
	}
	if deleted > 0 {
		slog.Debug("deleted expired webauthn sessions", "count", deleted)
	}

	return nil
}

func (a *API) RunRecommendationsJob(c AIConfig) error {
	slog.Debug("running recommendations job")

	candidateIDs, err := a.Store.GetCandidates(DefaultCandidatesBatchSize, DefaultRecommendationsJobFrequency)
	if err != nil {
		return fmt.Errorf("failed to get candidates: %w", err)
	}
	if len(candidateIDs) == 0 {
		slog.Debug("candidates list is empty, skipping run...")
		return nil
	}

	for i := range len(candidateIDs) {
		var positionIDs []uuid.UUID

		if c.UseEmbeddings {
			positionIDs, err = a.Store.GetPositionsForCandidateViaEmbeddings(candidateIDs[i], DefaultTopPositions)
			if err != nil {
				slog.Error("failed to find similar positions via embeddings", "candidateID", candidateIDs[i], "err", err)
				continue
			}
		} else {
			positionIDs, err = a.Store.GetPositionsForCandidateViaFTS(candidateIDs[i], DefaultTopPositions)
			if err != nil {
				slog.Error("failed to find similar positions via FTS", "candidateID", candidateIDs[i], "err", err)
				continue
			}
		}

		if len(positionIDs) == 0 {
			continue
		}

		if c.UseReranker {
			candidate, err := a.Store.GetCandidate(candidateIDs[i])
			if err != nil {
				slog.Error("failed to get candidate", "err", err)
				continue
			}

			positionsTexts, err := a.Store.GetPositionsTexts(positionIDs)
			if err != nil {
				slog.Error("failed to get position texts", "err", err)
				continue
			}

			results, err := CreateRerank(c, candidate.About, positionsTexts, positionIDs)
			if err != nil {
				slog.Error("failed to rerank", "candidateID", candidateIDs[i], "err", err)
				continue
			}

			limit := min(len(results), int(DefaultDailyRecommendations))
			for j := range limit {
				if _, err := a.Store.CreateRecommendation(positionIDs[results[j].Index], candidateIDs[i]); err != nil {
					return fmt.Errorf("failed to create recommendation: %w", err)
				}
			}

		} else {
			limit := min(len(positionIDs), int(DefaultDailyRecommendations))
			for j := range limit {
				if _, err := a.Store.CreateRecommendation(positionIDs[j], candidateIDs[i]); err != nil {
					return fmt.Errorf("failed to create recommendation: %w", err)
				}
			}
		}

	}

	return nil
}

func (a *API) RunEmbeddingsJob(c AIConfig) error {
	slog.Debug("running embeddings job")

	ids, texts, err := a.Store.FetchPendingEmbeddings(DefaultEmbeddingsBatchSize)
	if err != nil {
		return fmt.Errorf("failed to fetch pending embeddings: %w", err)
	}
	if len(ids) == 0 {
		slog.Debug("empty pending embeddings metadata, skipping run...")
		return nil
	}

	embeddings, err := CreateEmbeddings(c, texts)
	if err != nil {
		if err := a.Store.MarkEmbeddingsStatus(ids, EmbeddingStatusFailed); err != nil {
			return fmt.Errorf("failed to mark embeddings status: %w", err)
		}
		return fmt.Errorf("failed to create embeddings: %w", err)
	}

	tx, err := a.Store.Database.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	if err := a.Store.UpsertEmbeddingsTx(tx, ids, embeddings); err != nil {
		if err := tx.Rollback(); err != nil {
			slog.Error("failed to rollback transaction", "err", err)
			return fmt.Errorf("failed to rollback transaction: %w", err)
		}
		slog.Error("failed to upsert embeddings", "err", err)
		return fmt.Errorf("failed to upsert embeddings: %w", err)
	}

	if err := a.Store.MarkEmbeddingsStatusTx(tx, ids, EmbeddingStatusDone); err != nil {
		if err := tx.Rollback(); err != nil {
			slog.Error("failed to rollback transaction", "err", err)
			return fmt.Errorf("failed to rollback transaction: %w", err)
		}
		slog.Error("failed to mark embeddings status", "err", err)
		return fmt.Errorf("failed to mark embeddings status: %w", err)
	}

	return tx.Commit()
}

func (a *API) RunCleanupVisitors() {
	var expired []LimiterKey

	a.RateLimiter.Mu.RLock()

	now := time.Now().Unix()
	exp := int64(DefaultVisitorsInactivityFrame.Seconds())
	for key, visitor := range a.RateLimiter.Visitors {
		if now-visitor.LastSeen.Load() > exp {
			expired = append(expired, key)
		}
	}

	a.RateLimiter.Mu.RUnlock()

	if len(expired) == 0 {
		return
	}

	a.RateLimiter.Mu.Lock()

	now = time.Now().Unix()
	for _, key := range expired {
		visitor, exists := a.RateLimiter.Visitors[key]
		if !exists {
			continue
		}
		lastSeen := visitor.LastSeen.Load()
		if now-lastSeen > exp {
			delete(a.RateLimiter.Visitors, key)
		}
	}

	a.RateLimiter.Mu.Unlock()
}
