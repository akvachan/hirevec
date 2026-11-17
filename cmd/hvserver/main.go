// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package main

import (
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/akvachan/hirevec"
	"github.com/akvachan/hirevec/cmd/common"
)

func main() {
	if err := common.LoadEnv(".env"); err != nil {
		slog.Warn("could not load .env, using system environment", "err", err)
	}

	common.InitLogger(
		common.MustParseEnv("HIREVEC_LOG_LEVEL", common.ParseLogLevel, hirevec.DefaultLogLevel),
		common.MustParseEnv("HIREVEC_LOG_JSON", strconv.ParseBool, false),
	)

	if err := hirevec.RunApp(hirevec.AppConfig{
		LogLevel: common.MustParseEnv("HIREVEC_LOG_LEVEL", common.ParseLogLevel, hirevec.DefaultLogLevel),
		LogJSON:  common.MustParseEnv("HIREVEC_LOG_JSON", strconv.ParseBool, false),

		ServerURL:           common.MustParseEnv("HIREVEC_URL", url.Parse, &url.URL{Host: "localhost:6969"}),
		RequestReadTimeout:  common.MustParseEnv("HIREVEC_REQUEST_READ_TIMEOUT", time.ParseDuration, hirevec.DefaultRequestReadTimeout),
		RequestWriteTimeout: common.MustParseEnv("HIREVEC_REQUEST_WRITE_TIMEOUT", time.ParseDuration, hirevec.DefaultRequestWriteTimeout),
		GracePeriod:         common.MustParseEnv("HIREVEC_GRACE_PERIOD", time.ParseDuration, hirevec.DefaultGracePeriod),
		RateLimitingEnabled: common.MustParseEnv("HIREVEC_RATE_LIMITING_ENABLED", strconv.ParseBool, true),

		LlamaCppURL:             common.MustParseEnv("HIREVEC_LLAMACPP_URL", url.Parse, &url.URL{Host: ""}),
		LlamaCppEmbeddingsModel: os.Getenv("HIREVEC_LLAMACPP_EMBEDDINGS_MODEL"),
		LlamaCppEmbeddingsDim:   common.MustParseEnv("HIREVEC_LLAMACPP_EMBEDDINGS_DIM", hirevec.ParseEmbeddingsDim, 768),
		LlamaCppRerankerModel:   os.Getenv("HIREVEC_LLAMACPP_RERANKER_MODEL"),
		LlamaCppAPIKeyFile:      os.Getenv("HIREVEC_LLAMACPP_API_KEY_FILE"),

		EmbeddingsJobFrequency:      common.MustParseEnv("HIREVEC_EMBEDDINGS_JOB_FREQUENCY", time.ParseDuration, hirevec.DefaultEmbeddingsJobFrequency),
		RecommendationsJobFrequency: common.MustParseEnv("HIREVEC_RECOMMENDATIONS_JOB_FREQUENCY", time.ParseDuration, hirevec.DefaultRecommendationsJobFrequency),

		APIKeyFile: os.Getenv("HIREVEC_API_KEY_FILE"),

		WebAuthnRPDisplayName: os.Getenv("HIREVEC_WEBAUTHN_RP_DISPLAY_NAME"),
		WebAuthnRPID:          os.Getenv("HIREVEC_WEBAUTHN_RPID"),
		WebAuthnRPOrigins:     common.MustParseEnv("HIREVEC_WEBAUTHN_RP_ORIGINS", hirevec.ParseStringList, nil),

		SymmetricKeyFile:  os.Getenv("HIREVEC_SYMMETRIC_KEY_FILE"),
		AsymmetricKeyFile: os.Getenv("HIREVEC_ASYMMETRIC_KEY_FILE"),

		TLSEnabled:  common.MustParseEnv("HIREVEC_TLS_ENABLED", strconv.ParseBool, false),
		TLSCertFile: os.Getenv("HIREVEC_TLS_CERT_FILE"),
		TLSKeyFile:  os.Getenv("HIREVEC_TLS_KEY_FILE"),

		TrustedProxy: common.MustParseEnv("HIREVEC_TRUSTED_PROXY", strconv.ParseBool, false),
	}); err != nil {
		slog.Error(
			"failed to run app",
			"err", err,
		)
	}
}
