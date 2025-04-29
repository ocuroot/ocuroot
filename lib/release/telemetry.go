package release

import (
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
)

const name = "github.com/ocuroot/ocuroot/lib/release"

var (
	tracer = otel.Tracer(name)
	meter  = otel.Meter(name)
	logger = otelslog.NewLogger(name)
)

const (
	// Based on:
	// https://opentelemetry.io/docs/specs/semconv/cicd/cicd-spans/
	// https://opentelemetry.io/docs/specs/semconv/resource/cicd/
	// https://opentelemetry.io/docs/specs/semconv/registry/attributes/cicd/
	AttributeCICDPipelineName          = "cicd.pipeline.name"
	AttributeCICDPipelineActionName    = "cicd.pipeline.action.name"
	AttributeCICDPipelineRunID         = "cicd.pipeline.run.id"
	AttributeCICDPipelineResult        = "cicd.pipeline.result"
	AttributeCICDPipelineRunState      = "cicd.pipeline.run.state"
	AttributeCICDPipelineRunURL        = "cicd.pipeline.run.url.full"
	AttributeCICDPipelineRunURLShort   = "cicd.pipeline.run.url.short"
	AttributeCICDPipelineTaskName      = "cicd.pipeline.task.name"
	AttributeCICDPipelineTaskRunID     = "cicd.pipeline.task.run.id"
	AttributeCICDPipelineTaskRunType   = "cicd.pipeline.task.run.type"
	AttributeCICDPipelineTaskRunResult = "cicd.pipeline.task.run.result"
	AttributeErrorType                 = "error.type"
)
